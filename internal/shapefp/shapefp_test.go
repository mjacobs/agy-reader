package shapefp

import (
	"reflect"
	"strings"
	"testing"
)

// canonical is a test helper: canonicalize one or more raw JSON docs or fail.
func canonical(t *testing.T, docs ...string) []string {
	t.Helper()
	raw := make([][]byte, len(docs))
	for i, d := range docs {
		raw[i] = []byte(d)
	}
	lines, err := Canonicalize(raw)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	return lines
}

func TestBasicNestingAndTypes(t *testing.T) {
	got := canonical(t, `{"a":"x","b":{"c":1,"d":true},"e":null}`)
	want := []string{
		"\tobject", // root
		"a\tstring",
		"b\tobject",
		"b.c\tnumber",
		"b.d\tboolean",
		"e\tnull",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shape mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestArrayElementsUnion(t *testing.T) {
	// An array of heterogeneous elements collapses to one "[]" path whose
	// type-tags union. Objects inside contribute their fields once.
	got := canonical(t, `{"xs":[1,"two",{"k":true},null]}`)
	want := []string{
		"\tobject",
		"xs\tarray",
		"xs.[]\tnull|number|object|string",
		"xs.[].k\tboolean",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("array union mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestOptionalFieldUnionAcrossDocs(t *testing.T) {
	// A field present in one doc and absent in another must appear in the
	// union (not flip the fingerprint by presence/absence). And a leaf that is
	// a string in one doc and null in another unions its type-tags.
	lines := canonical(t,
		`{"step":{"a":"x","b":"present"}}`,
		`{"step":{"a":null}}`,
	)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "step.a\tnull|string") {
		t.Errorf("expected step.a to union null|string, got:\n%s", joined)
	}
	if !strings.Contains(joined, "step.b\tstring") {
		t.Errorf("expected optional step.b to survive in the union, got:\n%s", joined)
	}
	// Order of documents must not matter.
	rev := canonical(t,
		`{"step":{"a":null}}`,
		`{"step":{"a":"x","b":"present"}}`,
	)
	if !reflect.DeepEqual(lines, rev) {
		t.Errorf("document order changed the shape:\n a: %#v\n b: %#v", lines, rev)
	}
}

func TestVolatileKeyNormalization(t *testing.T) {
	// file URIs, filesystem paths, UUIDs, integers and long hex keys collapse
	// to "*", so a map keyed by data yields one shape regardless of its keys.
	got := canonical(t, `{"m":{
		"file:///home/u/proj":"rel",
		"file:///other/repo":"rel2",
		"550e8400-e29b-41d4-a716-446655440000":"v",
		"12345":"v",
		"deadbeefdeadbeef":"v",
		"realField":"v"
	}}`)
	want := []string{
		"\tobject",
		"m\tobject",
		"m.*\tstring",         // all the volatile keys collapse here
		"m.realField\tstring", // a normal identifier key is preserved
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("volatile-key mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestOpaqueSubtreesPruned(t *testing.T) {
	// The known config/passthrough blobs are recorded as leaves (presence +
	// container type) but never recursed, so their interior churn is invisible.
	doc := `{
		"generatorMetadata":{"whatever":{"deep":1}},
		"executorMetadatas":[{"config":{"flag":true}}],
		"metadata":{"mendelExperimentIds":["a","b"],"projectId":"p"},
		"steps":[
			{"generic":{"args":{"anyKey":123}},"metadata":{"internalMetadata":{"x":1},"sourceTrajectoryStepInfo":{"y":2},"createdAt":"t"}},
			{"userInput":{"userConfig":{"deep":{"more":1}},"activeUserState":{"z":1},"userResponse":"hi"}}
		]
	}`
	joined := strings.Join(canonical(t, doc), "\n")
	// Opaque nodes present as leaves...
	for _, leaf := range []string{
		"generatorMetadata\tobject",
		"executorMetadatas\tarray",
		"metadata.mendelExperimentIds\tarray",
		"steps.[].generic\tobject",
		"steps.[].metadata.internalMetadata\tobject",
		"steps.[].metadata.sourceTrajectoryStepInfo\tobject",
		"steps.[].userInput.userConfig\tobject",
		"steps.[].userInput.activeUserState\tobject",
	} {
		if !strings.Contains(joined, leaf) {
			t.Errorf("missing opaque leaf %q in:\n%s", leaf, joined)
		}
	}
	// ...but their interiors are NOT walked.
	for _, buried := range []string{
		"generatorMetadata.whatever",
		"executorMetadatas.[]",
		"metadata.mendelExperimentIds.[]",
		"steps.[].generic.args",
		"steps.[].metadata.internalMetadata.x",
		"steps.[].userInput.userConfig.deep",
	} {
		if strings.Contains(joined, buried) {
			t.Errorf("opaque interior %q leaked into the shape:\n%s", buried, joined)
		}
	}
	// Non-opaque siblings under the same parents are still captured.
	for _, kept := range []string{
		"metadata.projectId\tstring",
		"steps.[].metadata.createdAt\tstring",
		"steps.[].userInput.userResponse\tstring",
	} {
		if !strings.Contains(joined, kept) {
			t.Errorf("expected non-opaque field %q to be kept:\n%s", kept, joined)
		}
	}
}

func TestReorderStability(t *testing.T) {
	// Object key order and array element order (for the type-union) must not
	// change the canonical lines.
	a := canonical(t, `{"a":1,"b":{"c":2,"d":3},"xs":[1,2]}`)
	b := canonical(t, `{"xs":[2,1],"b":{"d":3,"c":2},"a":1}`)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("reorder changed shape:\n a: %#v\n b: %#v", a, b)
	}
}

func TestEnvelopeUnwrap(t *testing.T) {
	// The bare sidecar and the {"trajectory":{...}} RPC envelope must produce
	// the identical fingerprint.
	bare := `{"cascadeId":"x","steps":[{"type":"T"}]}`
	env := `{"trajectory":{"cascadeId":"x","steps":[{"type":"T"}]}}`
	fpBare, _, err := FingerprintDocs([][]byte{[]byte(bare)})
	if err != nil {
		t.Fatal(err)
	}
	fpEnv, _, err := FingerprintDocs([][]byte{[]byte(env)})
	if err != nil {
		t.Fatal(err)
	}
	if fpBare != fpEnv {
		t.Errorf("envelope unwrap mismatch: bare=%s env=%s", fpBare, fpEnv)
	}
}

func TestFingerprintFormatAndDeterminism(t *testing.T) {
	doc := `{"a":{"b":[1,2,3]},"c":"s"}`
	fp1, lines1, err := FingerprintDocs([][]byte{[]byte(doc)})
	if err != nil {
		t.Fatal(err)
	}
	fp2, _, err := FingerprintDocs([][]byte{[]byte(doc)})
	if err != nil {
		t.Fatal(err)
	}
	if fp1 != fp2 {
		t.Errorf("non-deterministic fingerprint: %s vs %s", fp1, fp2)
	}
	if !strings.HasPrefix(fp1, "sha256:") || len(fp1) != len("sha256:")+64 {
		t.Errorf("unexpected fingerprint format: %q", fp1)
	}
	// Fingerprint(lines) must equal FingerprintDocs's fingerprint.
	if got := Fingerprint(lines1); got != fp1 {
		t.Errorf("Fingerprint(lines) = %s, want %s", got, fp1)
	}
}

func TestInvalidJSON(t *testing.T) {
	if _, err := Canonicalize([][]byte{[]byte(`{not json`)}); err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}

func TestEmptyInput(t *testing.T) {
	// No documents => no lines => a stable fingerprint of the empty string.
	lines, err := Canonicalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %#v", lines)
	}
}
