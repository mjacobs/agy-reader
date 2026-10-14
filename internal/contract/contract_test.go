package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mjacobs/agy-reader/internal/contract"
)

func TestRecordedRenderFixturesValidateAgainstSchema(t *testing.T) {
	testdataDir := filepath.Join("..", "render", "testdata")
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		t.Fatalf("read testdata dir: %v", err)
	}

	validatedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".trajectory.json") {
			continue
		}
		path := filepath.Join(testdataDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			if err := contract.ValidateFile(path); err != nil {
				t.Fatalf("fixture %s failed contract validation: %v", entry.Name(), err)
			}
		})
		validatedCount++
	}

	if validatedCount < 7 {
		t.Fatalf("expected at least 7 fixtures validated, got %d", validatedCount)
	}
}

func TestValidSidecarWithAgyReader(t *testing.T) {
	valid := []byte(`{
		"cascadeId": "22222222-2222-2222-2222-222222222222",
		"trajectoryId": "11111111-1111-1111-1111-111111111111",
		"trajectoryType": "CORTEX_TRAJECTORY_TYPE_CASCADE",
		"source": "CORTEX_TRAJECTORY_SOURCE_CLI",
		"agyReader": {
			"parentCascadeId": "33333333-3333-3333-3333-333333333333",
			"futureField": {"preserved": true}
		},
		"steps": [
			{
				"type": "CORTEX_STEP_TYPE_USER_INPUT",
				"status": "CORTEX_STEP_STATUS_COMPLETED",
				"metadata": {
					"createdAt": "2026-01-01T00:00:00.000000000Z",
					"source": "CORTEX_STEP_SOURCE_USER"
				},
				"userInput": {
					"userResponse": "hello"
				}
			}
		]
	}`)
	if err := contract.ValidateBytes(valid); err != nil {
		t.Fatalf("expected valid sidecar to pass, got: %v", err)
	}
}

func TestValidSidecarWithLargeNumbers(t *testing.T) {
	// Sidecars with numbers beyond the float64 range (such as 1e400) in unrestricted
	// fields (e.g. unknown daemon fields, tool args, future extensions) must survive
	// validation without float64 overflow parsing errors.
	valid := []byte(`{
		"cascadeId": "22222222-2222-2222-2222-222222222222",
		"future": {"value": 1e400},
		"agyReader": {
			"futureNumber": 1e400
		},
		"steps": [
			{
				"type": "CORTEX_STEP_TYPE_USER_INPUT",
				"status": "CORTEX_STEP_STATUS_COMPLETED",
				"metadata": {
					"createdAt": "2026-01-01T00:00:00.000000000Z",
					"source": "CORTEX_STEP_SOURCE_USER"
				},
				"userInput": {
					"largeVal": 1e400
				}
			}
		]
	}`)
	if err := contract.ValidateBytes(valid); err != nil {
		t.Fatalf("expected sidecar with 1e400 numbers to validate successfully, got: %v", err)
	}
}

func TestInvalidSidecars(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name: "missing cascadeId",
			payload: `{
				"trajectoryId": "11111111-1111-1111-1111-111111111111",
				"steps": []
			}`,
		},
		{
			name: "non-uuid cascadeId",
			payload: `{
				"cascadeId": "not-a-uuid",
				"steps": []
			}`,
		},
		{
			name: "missing steps",
			payload: `{
				"cascadeId": "22222222-2222-2222-2222-222222222222"
			}`,
		},
		{
			name: "non-array steps",
			payload: `{
				"cascadeId": "22222222-2222-2222-2222-222222222222",
				"steps": "invalid"
			}`,
		},
		{
			name: "step missing metadata",
			payload: `{
				"cascadeId": "22222222-2222-2222-2222-222222222222",
				"steps": [
					{"type": "CORTEX_STEP_TYPE_USER_INPUT", "status": "CORTEX_STEP_STATUS_COMPLETED"}
				]
			}`,
		},
		{
			name: "step metadata missing createdAt",
			payload: `{
				"cascadeId": "22222222-2222-2222-2222-222222222222",
				"steps": [
					{
						"type": "CORTEX_STEP_TYPE_USER_INPUT",
						"status": "CORTEX_STEP_STATUS_COMPLETED",
						"metadata": {"source": "CORTEX_STEP_SOURCE_USER"}
					}
				]
			}`,
		},
		{
			name: "non-uuid parentCascadeId",
			payload: `{
				"cascadeId": "22222222-2222-2222-2222-222222222222",
				"agyReader": {"parentCascadeId": "bad-parent"},
				"steps": []
			}`,
		},
		{
			name: "trailing data after json object",
			payload: `{
				"cascadeId": "22222222-2222-2222-2222-222222222222",
				"steps": []
			} trailing_junk`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := contract.ValidateBytes([]byte(tc.payload)); err == nil {
				t.Fatalf("expected payload to fail validation, but it passed")
			}
		})
	}
}

func TestValidateFiles(t *testing.T) {
	testdataDir := filepath.Join("..", "render", "testdata")
	passed, failures := contract.ValidateFiles(testdataDir)
	if len(failures) > 0 {
		t.Fatalf("unexpected failures in testdata: %v", failures)
	}
	if passed < 7 {
		t.Fatalf("expected at least 7 files passed, got %d", passed)
	}
}
