package completeness

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestOfflineJSONQualifiedPresentation(t *testing.T) {
	for name, pair := range map[string][2]string{
		"view file":        {currentViewRaw, currentViewPresentation},
		"canceled command": {currentCanceledRaw, currentCanceledPresentation},
		"failed wrapper":   {wrapperErrorRaw, wrapperErrorPresentation},
	} {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, pair[0])
			before := s.Digest
			raw, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]any
			if err = json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal([]byte(pair[1]), &want); err != nil {
				t.Fatal(err)
			}
			delete(got, "agyReader")
			if !reflect.DeepEqual(got, want) {
				t.Fatal("offline export differs from qualified presentation")
			}
			if s.Digest != before {
				t.Fatal("export changed snapshot")
			}
			raw2, err := s.OfflineJSON()
			if err != nil || !bytes.Equal(raw, raw2) {
				t.Fatal("export is not repeatable")
			}
		})
	}
}

func TestOfflineJSONSourceOnlyBytes(t *testing.T) {
	payload := []byte{0x10, 0x00}
	s := sourceOnlySnapshot(t, genEdit(plannerField(payload...)))
	raw, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		AgyReader struct {
			Completeness json.RawMessage
			Offline      struct {
				Version          int
				Profile          string
				SourceDigest     string
				SourceOnlyFields []SourceOnlyField
			}
		}
	}
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	offline := value.AgyReader.Offline
	if value.AgyReader.Completeness != nil || offline.Version != 1 || offline.Profile != s.Profile || offline.SourceDigest != s.Digest {
		t.Fatal("incorrect offline provenance")
	}
	if len(offline.SourceOnlyFields) != 1 || len(offline.SourceOnlyFields[0].Occurrences) != 1 || !bytes.Equal(offline.SourceOnlyFields[0].Occurrences[0].Payload, payload) {
		t.Fatal("source-only bytes lost")
	}
}

func TestOfflineJSONRefusesUnsupportedProjection(t *testing.T) {
	raw := replace1216(t, currentViewRaw, `"args":{`, `"args":{"Future":"new",`)
	s := snapshot1216(t, raw)
	data, err := s.OfflineJSON()
	if err == nil || data != nil {
		t.Fatal("unsupported projection emitted a transcript")
	}
}
