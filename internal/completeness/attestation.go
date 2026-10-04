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
type Attestation struct {
	Profile        string `json:"profile"`
	SourceDigest   string `json:"sourceDigest"`
	PayloadDigest  string `json:"payloadDigest"`
	Representation string `json:"representation"`
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
	return &Attestation{Profile: Profile, SourceDigest: s.Digest, PayloadDigest: digest, Representation: representation}, r, false
}

// CheckAttestation rechecks a cached projection against the current snapshot.
// Missing, old or malformed evidence is unknown, never mtime-only verification.
func (s *Snapshot) CheckAttestation(presentation []byte, a *Attestation) Result {
	if s == nil {
		return Result{Unknown, "source snapshot unavailable"}
	}
	if a == nil || a.Profile != Profile || a.Representation != representation {
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
	return s.ComparePresentation(bytes.TrimSpace(compact.Bytes()))
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
