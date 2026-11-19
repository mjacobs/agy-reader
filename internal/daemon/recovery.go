package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mjacobs/agy-reader/internal/completeness"
	"google.golang.org/protobuf/encoding/protowire"
)

const responseLimit = completeness.MaxResponseBytes

// ErrRecovery marks a response that could not be assembled within the reader's
// budget. Callers may try a source-qualified transcript or back off the retry.
var ErrRecovery = errors.New("trajectory recovery failed")

type recordCounts struct {
	Steps      *int `json:"numTotalSteps"`
	Generators *int `json:"numTotalGeneratorMetadata"`
	Executors  *int `json:"numTotalExecutorMetadata"`
}

// completeLists resumes a truncated list at its exact offset. The caller checks
// the assembled raw and presented records against one persisted snapshot.
func (c *Client) completeLists(ctx context.Context, id string, full bool, fields map[string]json.RawMessage, counts recordCounts) (bool, error) {
	changed := false
	budget := responseLimit
	for k, v := range fields {
		budget -= len(k) + len(v) + 4
	}
	for _, spec := range []struct {
		field, method, offset, result string
		total                         *int
	}{
		{"steps", "GetCascadeTrajectorySteps", "stepOffset", "steps", counts.Steps},
		{"generatorMetadata", "GetCascadeTrajectoryGeneratorMetadata", "generatorMetadataOffset", "generatorMetadata", counts.Generators},
		{"executorMetadatas", "GetCascadeTrajectoryExecutorMetadatas", "executorMetadataOffset", "executorMetadata", counts.Executors},
	} {
		if spec.total == nil {
			continue
		}
		target := *spec.total
		if target < 0 || target > 100000 {
			return false, errors.New("invalid trajectory record count")
		}
		var rows []json.RawMessage
		if data, ok := fields[spec.field]; ok {
			if err := json.Unmarshal(data, &rows); err != nil {
				return false, err
			}
		}
		if len(rows) > target {
			return false, errors.New("trajectory exceeds advertised record count")
		}
		if len(rows) == target {
			continue
		}
		for len(rows) < target {
			args := map[string]any{"cascadeId": id, spec.offset: len(rows)}
			if spec.field == "steps" && full {
				args["trajectoryVerbosity"] = "CLIENT_TRAJECTORY_VERBOSITY_FULL"
				args["disableRehydration"] = true
			}
			if spec.field == "generatorMetadata" {
				args["includeMessages"] = true
			}
			payload, err := json.Marshal(args)
			if err != nil {
				return false, err
			}
			if budget <= 0 {
				return false, errors.New("recovered trajectory exceeds size limit")
			}
			page, err := c.request(ctx, spec.method, "application/json", payload, budget)
			if err != nil {
				return false, err
			}
			var envelope map[string][]json.RawMessage
			if err = json.Unmarshal(page, &envelope); err != nil {
				return false, err
			}
			next := envelope[spec.result]
			if len(next) == 0 || len(next) > target-len(rows) {
				return false, fmt.Errorf("%s returned invalid page length", spec.method)
			}
			for _, row := range next {
				budget -= len(row) + 1
			}
			rows = append(rows, next...)
		}
		data, err := json.Marshal(rows)
		if err != nil {
			return false, err
		}
		fields[spec.field] = data
		changed = true
	}
	return changed, nil
}

// GetTranscript uses the compact binary envelope because its JSON rendering
// can exceed the response bound even when the daemon capped the protobuf.
func (c *Client) GetTranscript(ctx context.Context, id string, full bool, steps, generators, executors int) (*Trajectory, error) {
	req := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), id)
	if full {
		req = protowire.AppendVarint(protowire.AppendTag(req, 3, protowire.VarintType), 3)
		req = protowire.AppendVarint(protowire.AppendTag(req, 4, protowire.VarintType), 1)
	}
	data, err := c.request(ctx, "GetCascadeTrajectory", "application/proto", req, responseLimit)
	if err != nil {
		return nil, err
	}
	header, err := transcriptHeader(data, [3]int{steps, generators, executors})
	if err != nil {
		return nil, err
	}
	fields, err := completeness.TranscriptHeaderJSON(header)
	if err != nil {
		return nil, err
	}
	if _, err = c.completeLists(ctx, id, full, fields, recordCounts{Steps: &steps, Generators: &generators}); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	if len(raw) > responseLimit {
		return nil, errors.New("transcript exceeds size limit")
	}
	return trajectoryFromJSON(raw)
}

func transcriptHeader(data []byte, expected [3]int) ([]byte, error) {
	var trajectory []byte
	var totals [3]int
	bad := errors.New("invalid trajectory protobuf envelope")
	seen := map[protowire.Number]bool{}
	for len(data) > 0 {
		n, t, k := protowire.ConsumeTag(data)
		if k < 0 || seen[n] {
			return nil, bad
		}
		seen[n] = true
		data = data[k:]
		switch {
		case n == 1 && t == protowire.BytesType:
			v, k := protowire.ConsumeBytes(data)
			if k < 0 {
				return nil, bad
			}
			trajectory = v
			data = data[k:]
		case n >= 2 && n <= 5 && t == protowire.VarintType:
			v, k := protowire.ConsumeVarint(data)
			if k < 0 || v > 1<<32-1 {
				return nil, bad
			}
			if n >= 3 {
				totals[n-3] = int(v)
			}
			data = data[k:]
		default:
			return nil, bad
		}
	}
	if trajectory == nil || totals != expected {
		return nil, errors.New("daemon trajectory totals differ from source")
	}
	// The dedicated RPCs supply complete step and generator lists. Executor
	// records are outside the transcript representation.
	var header []byte
	for len(trajectory) > 0 {
		n, t, k := protowire.ConsumeField(trajectory)
		if k < 0 {
			return nil, bad
		}
		switch n {
		case 2, 3, 9:
			if t != protowire.BytesType {
				return nil, bad
			}
		default:
			header = append(header, trajectory[:k]...)
		}
		trajectory = trajectory[k:]
	}
	return header, nil
}
