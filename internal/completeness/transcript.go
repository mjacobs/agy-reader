package completeness

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
)

const TranscriptReason = "verified transcript only; executor metadata omitted"

const MaxResponseBytes = maxBytes

func (s *Snapshot) GeneratorCount() int {
	m := s.trajectory.ProtoReflect()
	return m.Get(m.Descriptor().Fields().ByName("generator_metadata")).List().Len()
}

// TranscriptHeaderJSON uses the pinned trajectory descriptor to preserve the
// header independently of the daemon's truncated repeated record fields.
func TranscriptHeaderJSON(header []byte) (map[string]json.RawMessage, error) {
	m, err := decode(trajectoryName, header)
	if err != nil {
		return nil, err
	}
	raw, err := (protojson.MarshalOptions{Resolver: schemaTypes}).Marshal(m)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	err = json.Unmarshal(raw, &fields)
	return fields, err
}
