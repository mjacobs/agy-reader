package completeness

import (
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
)

// OfflineJSON reconstructs the supported daemon presentation from persisted
// records. It does not contact a daemon or produce a completeness attestation.
// Declared source-only bytes remain available separately from the presentation.
func (s *Snapshot) OfflineJSON() ([]byte, error) {
	projected, err := project(s.trajectory)
	if err != nil {
		return nil, fmt.Errorf("offline presentation: %w", err)
	}
	raw, err := (protojson.MarshalOptions{Resolver: schemaTypes}).Marshal(projected)
	if err != nil {
		return nil, fmt.Errorf("offline JSON: %w", err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if _, ok := fields["steps"]; !ok {
		fields["steps"] = json.RawMessage(`[]`)
	}
	metadata := struct {
		Offline struct {
			Version          int               `json:"version"`
			Profile          string            `json:"profile"`
			SourceDigest     string            `json:"sourceDigest"`
			SourceOnlyFields []SourceOnlyField `json:"sourceOnlyFields,omitempty"`
		} `json:"offline"`
	}{}
	metadata.Offline.Version = 1
	metadata.Offline.Profile = s.Profile
	metadata.Offline.SourceDigest = s.Digest
	metadata.Offline.SourceOnlyFields = s.sourceOnly
	fields["agyReader"], err = json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(fields, "", "  ")
}
