package completeness

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

// Attestation records a verified supported daemon projection. It is not a claim
// that the sidecar archives the generic wrapper fields omitted by the daemon.
// Parent-link metadata is outside the payload digest and cannot mint evidence.
//
// When the source carries declared source-only content, Reason is
// SourceOnlyReason and SourceOnly records that content; both must match the
// snapshot on every recheck.
type Attestation struct {
	Profile        string            `json:"profile"`
	SourceDigest   string            `json:"sourceDigest"`
	PayloadDigest  string            `json:"payloadDigest"`
	Representation string            `json:"representation"`
	Reason         string            `json:"reason,omitempty"`
	SourceOnly     []SourceOnlyField `json:"sourceOnly,omitempty"`
}

const representation = "supported-daemon-projection"

func PayloadDigest(raw []byte) (string, error) {
	var top map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&top); err != nil || top == nil {
		return "", errors.New("invalid trajectory JSON")
	}
	delete(top, "agyReader")
	data, err := json.Marshal(top)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Snapshot) VerifyResponses(fullRaw, presentation []byte) (*Attestation, Result) {
	a, r, _ := s.Verify(fullRaw, presentation)
	return a, r
}

// Verify is VerifyResponses that also reports whether a refusal is
// source-proven: fixed by the snapshot and Profile alone, so that retrying the
// same snapshot refuses it whatever the daemon answers. Only a projection of
// the snapshot that the profile does not support is source-proven; a refusal
// that comes from validating or comparing the daemon's responses is not.
func (s *Snapshot) Verify(fullRaw, presentation []byte) (a *Attestation, r Result, sourceProven bool) {
	r, sourceProven = s.compareResponses(fullRaw, presentation)
	if r.State != Verified {
		return nil, r, sourceProven
	}
	digest, err := PayloadDigest(presentation)
	if err != nil {
		return nil, Result{Unknown, "invalid trajectory JSON"}, false
	}
	a = &Attestation{Profile: s.Profile, SourceDigest: s.Digest, PayloadDigest: digest, Representation: representation}
	if len(s.sourceOnly) > 0 {
		a.Reason, a.SourceOnly = SourceOnlyReason, s.sourceOnly
		r = Result{Verified, SourceOnlyReason}
	}
	return a, r, false
}

// sourceOnlyMatches compares the attestation's source-only record with the
// snapshot's, through their JSON form so that an empty payload read back from
// a sidecar equals one computed from the source.
func (s *Snapshot) sourceOnlyMatches(a *Attestation) bool {
	want := ""
	if len(s.sourceOnly) > 0 {
		want = SourceOnlyReason
	}
	if a.Reason != want {
		return false
	}
	got, err1 := json.Marshal(a.SourceOnly)
	expected, err2 := json.Marshal(s.sourceOnly)
	return err1 == nil && err2 == nil && bytes.Equal(got, expected)
}

// CheckAttestation rechecks a cached projection against the current snapshot.
// Missing, old or malformed evidence is unknown, never mtime-only verification.
func (s *Snapshot) CheckAttestation(presentation []byte, a *Attestation) Result {
	if s == nil {
		return Result{Unknown, "source snapshot unavailable"}
	}
	if a == nil || a.Profile != s.Profile || a.Representation != representation {
		return Result{Unknown, "missing or unsupported completeness attestation"}
	}
	digest, err := PayloadDigest(presentation)
	if err != nil {
		return Result{Unknown, "invalid trajectory JSON"}
	}
	if a.SourceDigest != s.Digest {
		return Result{Incomplete, "persisted source content changed"}
	}
	if a.PayloadDigest != digest {
		return Result{Incomplete, "sidecar payload changed since verification"}
	}
	if !s.sourceOnlyMatches(a) {
		return Result{Incomplete, "sidecar source-only record changed since verification"}
	}
	// A sidecar is re-indented and may HTML-escape every string when reader
	// metadata is added. Apply the RPC bound to its compact payload, not that
	// serialization overhead or the reader-owned namespace.
	var top map[string]any
	decoder := json.NewDecoder(bytes.NewReader(presentation))
	decoder.UseNumber()
	if err := decoder.Decode(&top); err != nil || top == nil {
		return Result{Unknown, "invalid trajectory JSON"}
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Result{Unknown, "invalid trajectory JSON"}
	}
	delete(top, "agyReader")
	var compact bytes.Buffer
	encoder := json.NewEncoder(&compact)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(top); err != nil {
		return Result{Unknown, "invalid trajectory JSON"}
	}
	r := s.ComparePresentation(bytes.TrimSpace(compact.Bytes()))
	if r.State == Verified && len(s.sourceOnly) > 0 {
		r.Reason = SourceOnlyReason
	}
	return r
}

func ReadAttestation(raw []byte) *Attestation {
	var top struct {
		Reader struct {
			Completeness *Attestation `json:"completeness"`
		} `json:"agyReader"`
	}
	if json.Unmarshal(raw, &top) != nil {
		return nil
	}
	return top.Reader.Completeness
}
