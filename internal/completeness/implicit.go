package completeness

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
)

// ImplicitProfile identifies agy's background activity records in implicit/.
const ImplicitProfile = "implicit-trajectory-v1/" + LegacyProfile

// The 1.5.16 exa.jetski_cortex_pb.ImplicitTrajectory envelope contains one
// trajectory (field 1) and its workspace scope (field 5). Reject unknown,
// duplicate or missing fields before interpreting the embedded trajectory.
func implicitTrajectory(data []byte) ([]byte, json.RawMessage, error) {
	var trajectory, scope []byte
	for len(data) > 0 {
		number, typ, n := protowire.ConsumeTag(data)
		if n < 0 || typ != protowire.BytesType {
			return nil, nil, refusal("invalid implicit trajectory envelope")
		}
		value, size := protowire.ConsumeBytes(data[n:])
		if size < 0 {
			return nil, nil, refusal("truncated implicit trajectory envelope")
		}
		switch {
		case number == 1 && trajectory == nil:
			trajectory = value
		case number == 5 && scope == nil:
			scope = value
		default:
			return nil, nil, refusal("unsupported implicit trajectory envelope")
		}
		data = data[n+size:]
	}
	if len(trajectory) == 0 || scope == nil {
		return nil, nil, refusal("missing implicit trajectory or scope")
	}
	m, err := decode("exa.cortex_pb.TrajectoryScope", scope)
	if err != nil {
		return nil, nil, refuse(err)
	}
	scopeJSON, err := protojson.Marshal(m)
	return trajectory, scopeJSON, err
}
