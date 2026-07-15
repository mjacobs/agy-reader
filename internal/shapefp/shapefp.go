// Package shapefp computes a canonical "shape fingerprint" of the trajectory
// JSON that agy-reader writes into <uuid>.trajectory.json sidecars.
//
// # Why this exists
//
// The agy-format-audit skill fingerprints the SQLite .db schema, but the
// sidecar content preserves the raw GetCascadeTrajectory JSON-RPC trajectory
// values (internal/cache/sidecar.go). Google can reshape that RPC response —
// rename a step-payload field, restructure a diff, drop a value — WITHOUT ever
// touching the .db schema. The schema fingerprint would stay UNCHANGED while
// agentsview's rendering silently degrades (its parsers tolerate unknown
// fields, so nothing alarms). This package closes that blind spot by
// fingerprinting the *structure* of the trajectory JSON that agentsview
// actually renders.
//
// # The algorithm (precise; reimplementable byte-identically)
//
// A fingerprint is sha256 over a deterministic, order-insensitive listing of
// the JSON's key-structure — field names and value *types*, values excluded.
// It is designed so agentsview (or any consumer) can recompute the exact same
// digest from the same sidecars.
//
//  1. Parse each sidecar to a generic JSON value. If the document is the RPC
//     envelope {"trajectory": {...}} rather than a bare Trajectory, unwrap it.
//  2. Remove the root agyReader member, when present. That namespace is
//     agy-reader-owned metadata, not daemon response shape, and contributes no
//     node or child paths to the fingerprint.
//  3. Walk the remaining value. Every node (including the root, whose path is
//     the empty string) contributes one entry: path -> type-tag.
//     - type-tag is the node's JSON type: object | array | string | number |
//     boolean | null.
//     - path is a dot-joined sequence of segments from the root:
//     * an object member contributes its key as a segment, UNLESS the key is
//     "volatile" (data-dependent — a file:// URI, a filesystem path, a
//     UUID, a pure integer, or a long hex id), in which case the segment
//     is normalized to "*". This collapses maps that are keyed by data
//     (e.g. workspaceUrisToRelativePaths, keyed by absolute file URIs) so
//     they don't explode the fingerprint per-workspace/per-machine.
//     * an array element contributes the segment "[]". All elements of an
//     array therefore share one path, and their type-tags union (see 4).
//  4. Opaque subtrees are pruned: at a fixed set of paths (see opaquePaths),
//     the node's own entry is emitted but its children are NOT walked. These
//     are config/passthrough blobs that agy-reader stores as json.RawMessage
//     and never structurally models, that carry open-ended data-dependent keys
//     (config keyed by binary name, tool args keyed by parameter name), and
//     that agentsview does not schema-render. Including their interiors would
//     swamp the fingerprint with config noise that flips on every agy settings
//     or version change while telling you nothing about render fidelity — and
//     would break cross-machine reproducibility (feature-dependent config is
//     present only in sessions that used the feature).
//  5. Entries are unioned across ALL supplied sidecars and across every
//     occurrence of a path (array elements, "*"-collapsed keys, repeated
//     documents). A path maps to the SET of type-tags seen for it; the set is
//     sorted and joined with "|" (e.g. "null|number" for an optional-null
//     leaf). Unioning across all sidecars is the key false-DRIFT defense:
//     optional fields present in one session and absent in another both land
//     in the union, so their presence/absence does not flip the digest.
//  6. Serialize: one line per path, formatted exactly as
//     path + "\t" + typeUnion. Sort the lines by byte order (Go's
//     sort.Strings). Join with "\n" (no trailing newline). The fingerprint is
//     "sha256:" + lowercase-hex(sha256(that string)).
//
// # Residual noise (honest limits)
//
// The union-across-sidecars defense is only as complete as the corpus. A path
// that no supplied session ever exercises cannot appear in the union, so two
// machines whose sidecar corpora exercise different optional features can
// produce different fingerprints at the SAME agy version. Empirically (89 CLI
// sidecars, mid-2026) the residual after pruning is tiny — a couple of
// optional attachment/edit fields — versus ~78% of raw paths being config
// noise that pruning removes. Treat a DRIFT the way the schema fingerprint is
// treated: a signal that *something* moved, to be triaged against the agy
// changelog, not an automatic failure. The recorded baseline should be
// captured from a broad, representative corpus so the union is as complete as
// possible.
package shapefp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// opaquePaths are the normalized paths whose node is recorded (presence +
// container type) but whose children are not walked. See the package doc for
// the rationale. Kept as an explicit, sorted, documented constant so a
// reimplementation (agentsview) can mirror it byte-for-byte.
//
// The mapping to internal/daemon/types.go json.RawMessage passthrough fields:
//
//	generatorMetadata                        Trajectory.GeneratorMetadata
//	executorMetadatas                        Trajectory.ExecutorMetadatas
//	metadata.mendelExperimentIds             TrajectoryMetadata.MendelExperimentIDs
//	steps.[].generic                         Step.Generic (raw-dumped, not schema-rendered)
//	steps.[].userInput.userConfig            UserInput.UserConfig (per-user config snapshot)
//	steps.[].userInput.activeUserState       UserInput.ActiveUserState (user-state snapshot)
//	steps.[].metadata.sourceTrajectoryStepInfo StepMetadata.SourceTrajectoryStepInfo
//	steps.[].metadata.internalMetadata       StepMetadata.InternalMetadata
var opaquePaths = map[string]bool{
	"executorMetadatas":                          true,
	"generatorMetadata":                          true,
	"metadata.mendelExperimentIds":               true,
	"steps.[].generic":                           true,
	"steps.[].metadata.internalMetadata":         true,
	"steps.[].metadata.sourceTrajectoryStepInfo": true,
	"steps.[].userInput.activeUserState":         true,
	"steps.[].userInput.userConfig":              true,
}

// volatileKey matches object keys that are data (not schema): file URIs,
// filesystem paths, UUIDs, pure integers, and long hex ids. Such a key is
// normalized to "*" so a map keyed by data contributes one shape, not one per
// key. The patterns are intentionally simple and anchored so they are trivial
// to reimplement identically.
var volatileKey = regexp.MustCompile(
	`^file:` + // a file:// URI (workspace map keys)
		`|/` + // contains a path separator
		`|^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$` + // UUID
		`|^[0-9]+$` + // pure integer
		`|^[0-9a-f]{16,}$`, // long lowercase hex id
)

func normalizeKey(k string) string {
	if volatileKey.MatchString(k) {
		return "*"
	}
	return k
}

func joinPath(parent, seg string) string {
	if parent == "" {
		return seg
	}
	return parent + "." + seg
}

func typeTag(v any) string {
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case float64, json.Number:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	default:
		// Unreachable for values produced by encoding/json, but keep the
		// fingerprint total rather than panicking on an unexpected type.
		return "unknown"
	}
}

// walk records the node at path and, unless the path is opaque, recurses.
// shape maps a path to the set of type-tags observed for it.
func walk(path string, v any, shape map[string]map[string]bool) {
	tag := typeTag(v)
	tags, ok := shape[path]
	if !ok {
		tags = map[string]bool{}
		shape[path] = tags
	}
	tags[tag] = true

	if opaquePaths[path] {
		return
	}

	switch t := v.(type) {
	case map[string]any:
		for k, cv := range t {
			// agyReader is reader-owned sidecar metadata, not daemon response
			// shape. Skip the root subtree without emitting even its node so
			// adding or changing metadata is fingerprint-invisible.
			if path == "" && k == "agyReader" {
				continue
			}
			walk(joinPath(path, normalizeKey(k)), cv, shape)
		}
	case []any:
		child := joinPath(path, "[]")
		for _, e := range t {
			walk(child, e, shape)
		}
	}
}

func unwrapEnvelope(v any) any {
	// The bare sidecar is a Trajectory. The live RPC response wraps it as
	// {"trajectory": {...}}. Accept either so the same fingerprint can be
	// computed from a sidecar on disk or a raw daemon response.
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if len(m) == 1 {
		if inner, ok := m["trajectory"]; ok {
			return inner
		}
	}
	return v
}

// Canonicalize parses each JSON document, walks its shape, unions the shapes,
// and returns the sorted canonical lines ("path\ttypeUnion"). Documents that
// fail to parse return an error identifying the index.
func Canonicalize(docs [][]byte) ([]string, error) {
	shape := map[string]map[string]bool{}
	for i, doc := range docs {
		var v any
		if err := json.Unmarshal(doc, &v); err != nil {
			return nil, fmt.Errorf("shapefp: document %d: %w", i, err)
		}
		walk("", unwrapEnvelope(v), shape)
	}
	return linesFromShape(shape), nil
}

func linesFromShape(shape map[string]map[string]bool) []string {
	lines := make([]string, 0, len(shape))
	for path, tags := range shape {
		union := make([]string, 0, len(tags))
		for t := range tags {
			union = append(union, t)
		}
		sort.Strings(union)
		lines = append(lines, path+"\t"+strings.Join(union, "|"))
	}
	sort.Strings(lines)
	return lines
}

// Fingerprint returns "sha256:<hex>" over the canonical lines. It is the
// caller's contract that lines came from Canonicalize (sorted, "\t"-joined).
func Fingerprint(lines []string) string {
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// FingerprintDocs is the one-shot convenience: Canonicalize then Fingerprint.
func FingerprintDocs(docs [][]byte) (string, []string, error) {
	lines, err := Canonicalize(docs)
	if err != nil {
		return "", nil, err
	}
	return Fingerprint(lines), lines, nil
}
