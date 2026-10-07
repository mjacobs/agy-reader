package completeness

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// This synthetic fixture reproduces a successful generate_image call stored as
// a generic wrapper over an embedded native GENERATE_IMAGE Step
// (agy-reader#ntzm). A frozen source, full RPC and default RPC copy of the
// retained conversation showed that the default response presents the native
// Step unchanged: the wrapper's outer status, arguments, result text, media and
// output URI are dropped, and the native Step keeps its own metadata. The
// wrapper's one media item repeats the native generated media. Values are
// synthetic; the expected presentation is written out explicitly.
const imageDoneArguments = `{\"AspectRatio\":\"16:9\",\"ImageName\":\"harbor\",\"Prompt\":\"Draw a harbor\",\"toolAction\":\"Drawing harbor\",\"toolSummary\":\"Harbor illustration\"}`
const imageDoneNative = `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-image-done","name":"generate_image","argumentsJson":"` + imageDoneArguments + `"},"completedAt":"2026-01-01T00:00:01Z"},"generateImage":{"prompt":"Draw a harbor","imageName":"harbor","aspectRatio":"16:9","modelName":"fixture-image-model","generatedMedia":{"mimeType":"image/png","uri":"file:///fixture/artifacts/harbor.png"}}`
const imageDoneRaw = `{"cascadeId":"image-done-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"id":"call-image-done","name":"generate_image","argumentsJson":"` + imageDoneArguments + `"},"completedAt":"2026-01-01T00:00:02Z"},"generic":{"args":{"AspectRatio":"16:9","ImageName":"harbor","Prompt":"Draw a harbor","toolAction":"Drawing harbor","toolSummary":"Harbor illustration"},"result":{"result":"Image saved","fullOutputUri":"file:///fixture/steps/1/output.txt","media":[{"mimeType":"image/png","uri":"file:///fixture/artifacts/harbor.png"}],"payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + imageDoneNative + `}}}}]}`
const imageDonePresentation = `{"cascadeId":"image-done-fixture","metadata":{},"steps":[{` + imageDoneNative + `}]}`

func TestProjectionSuccessfulEmbeddedImage(t *testing.T) {
	require1216(t, snapshot1216(t, imageDoneRaw), imageDoneRaw, imageDonePresentation, Verified)
}

// A source mutation applies to the persisted trajectory and both RPC copies; the
// presentation follows only where it carries the same content.
func TestProjectionSuccessfulEmbeddedImageChanges(t *testing.T) {
	const outerDone = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE"`
	const innerDone = `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_DONE"`
	const wrapperMedia = `"media":[{"mimeType":"image/png","uri":"file:///fixture/artifacts/harbor.png"}]`
	const nativeMedia = `"generatedMedia":{"mimeType":"image/png","uri":"file:///fixture/artifacts/harbor.png"}`
	for _, tc := range []struct {
		name, before, after, where string
		want                       State
	}{
		// changed media
		{"wrapper media uri differs from native", wrapperMedia, `"media":[{"mimeType":"image/png","uri":"file:///fixture/artifacts/other.png"}]`, "wrapper", Unknown},
		{"wrapper media mime type differs from native", wrapperMedia, `"media":[{"mimeType":"image/jpeg","uri":"file:///fixture/artifacts/harbor.png"}]`, "wrapper", Unknown},
		{"wrapper media dropped", wrapperMedia + `,`, ``, "wrapper", Unknown},
		{"wrapper media doubled", wrapperMedia, `"media":[{"mimeType":"image/png","uri":"file:///fixture/artifacts/harbor.png"},{"mimeType":"image/png","uri":"file:///fixture/artifacts/harbor.png"}]`, "wrapper", Unknown},
		{"native media dropped", `,` + nativeMedia, ``, "source", Unknown},
		{"native media uri changed", `artifacts/harbor.png"}}`, `artifacts/other.png"}}`, "native", Unknown},
		{"presented media changed", nativeMedia, `"generatedMedia":{"mimeType":"image/png","uri":"file:///fixture/artifacts/other.png"}`, "presentation", Incomplete},
		{"presented media dropped", `,` + nativeMedia, ``, "presentation", Incomplete},
		// changed argument
		{"wrapper prompt differs from native", `"Prompt":"Draw a harbor","toolAction"`, `"Prompt":"Draw a quay","toolAction"`, "wrapper", Unknown},
		{"wrapper image name differs from native", `"ImageName":"harbor","Prompt"`, `"ImageName":"quay","Prompt"`, "wrapper", Unknown},
		{"wrapper aspect ratio differs from native", `"AspectRatio":"16:9","ImageName"`, `"AspectRatio":"1:1","ImageName"`, "wrapper", Unknown},
		{"wrapper argument dropped", `"AspectRatio":"16:9","ImageName":"harbor",`, `"ImageName":"harbor",`, "wrapper", Unknown},
		{"unknown wrapper argument", `"args":{`, `"args":{"Future":"new",`, "source", Unknown},
		{"unrelated known wrapper argument", `"args":{`, `"args":{"CommandLine":"echo unsafe",`, "source", Unknown},
		{"presented prompt changed", `"prompt":"Draw a harbor"`, `"prompt":"Draw a quay"`, "presentation", Incomplete},
		{"presented model changed", `"modelName":"fixture-image-model"`, `"modelName":"other-model"`, "presentation", Incomplete},
		{"presented aspect ratio changed", `"aspectRatio":"16:9"`, `"aspectRatio":"1:1"`, "presentation", Incomplete},
		// unknown fields
		{"unknown wrapper result field", `"media":[`, `"stepRenderInfo":{"titlePrefix":"Generated"},"media":[`, "wrapper", Unknown},
		{"unknown native step field", `"generateImage":{`, `"taskDetails":{"id":"image-done-fixture/task-1"},"generateImage":{`, "native", Unknown},
		{"unexpected wrapper field", `"generic":{`, `"permissions":{},"generic":{`, "wrapper", Unknown},
		// changed status
		{"wrapper status not done", outerDone, `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_CANCELED"`, "wrapper", Unknown},
		{"native status not done", innerDone, `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_RUNNING"`, "native", Unknown},
		{"native status error without error", innerDone, `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_ERROR"`, "native", Unknown},
		{"native error on a done step", `"generateImage":{`, `"error":{"shortError":"boom"},"generateImage":{`, "native", Unknown},
		{"presented status changed", innerDone, `"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_ERROR"`, "presentation", Incomplete},
		// another tool
		{"embedded image step on another tool", `"name":"generate_image"`, `"name":"edit_image"`, "source", Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, presentation := imageDoneRaw, imageDonePresentation
			switch tc.where {
			case "source":
				raw = replace1216(t, raw, tc.before, tc.after)
				presentation = strings.ReplaceAll(presentation, tc.before, tc.after)
			case "wrapper":
				// The wrapper alone changes: its dropped copy no longer matches
				// the native Step, which the presentation still shows.
				raw = replace1216(t, raw, tc.before, tc.after)
			case "native":
				// Only the embedded native Step changes, so the presentation
				// would follow but the wrapper's own copy no longer matches.
				before := `"payload":{"@type":"type.googleapis.com/gemini_coder.Step",` + imageDoneNative
				after := replace1216(t, before, tc.before, tc.after)
				raw = replace1216(t, raw, before, after)
			case "presentation":
				presentation = replace1216(t, presentation, tc.before, tc.after)
			}
			s := snapshot1216(t, raw)
			require1216(t, s, raw, presentation, tc.want)
		})
	}
}

// An unknown wire field in the persisted payload, on the embedded Step or on
// its image, is refused when the snapshot is built.
func TestProjectionSuccessfulEmbeddedImageUnknownFields(t *testing.T) {
	for _, nested := range []bool{false, true} {
		m := trajectory1216(t, imageDoneRaw)
		payload := message1216(step1216(m), "generic", "result", "payload")
		f := payload.Descriptor().Fields()
		native, err := newMessage("gemini_coder.Step")
		if err != nil {
			t.Fatal(err)
		}
		if err := proto.Unmarshal(payload.Get(f.ByName("value")).Bytes(), native); err != nil {
			t.Fatal(err)
		}
		target := native.ProtoReflect()
		if nested {
			target = target.Get(target.Descriptor().Fields().ByName("generate_image")).Message()
		}
		target.SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 50000, protowire.VarintType), 1))
		value, err := proto.Marshal(native)
		if err != nil {
			t.Fatal(err)
		}
		payload.Set(f.ByName("value"), protoreflect.ValueOfBytes(value))
		if _, err := finishSnapshot(m); err == nil {
			t.Fatalf("unknown field accepted (nested image field: %v)", nested)
		}
	}
}
