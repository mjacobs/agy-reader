package completeness

import (
	"bytes"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// These synthetic fixtures model the three additional 1.2.16 projection shapes.
// Expected presentations are written explicitly so a projection regression cannot
// change both the actual and expected results together.
const syntheticInvokeRaw = `{"cascadeId":"parent-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"invoke_subagent"},"completedAt":"2026-01-01T00:00:02Z"},"taskDetails":{"id":"task-fixture"},"generic":{"args":{"Subagents":"[{\"TypeName\":\"image-generator\",\"Role\":\"Illustrator\",\"Prompt\":\"Draw a bird\",\"Model\":\"inherit\"}]","toolAction":"Invoking illustrator","toolSummary":"Illustration"},"result":{"result":"Created child-fixture","fullOutputUri":"file:///fixture/parent/output.txt","payload":{"@type":"type.googleapis.com/gemini_coder.Step","type":"CORTEX_STEP_TYPE_INVOKE_SUBAGENT","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"invoke_subagent"},"completedAt":"2026-01-01T00:00:01Z"},"invokeSubagent":{"subagents":[{"typeName":"image-generator","role":"Illustrator","initialPrompt":"Draw a bird","inherit":true}],"results":[{"conversationId":"child-fixture","logAbsoluteUri":"file:///fixture/child/transcript.jsonl"}]}}}}}]}`
const syntheticInvokePresentation = `{"cascadeId":"parent-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_INVOKE_SUBAGENT","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"invoke_subagent"},"completedAt":"2026-01-01T00:00:01Z"},"taskDetails":{"id":"task-fixture"},"invokeSubagent":{"subagents":[{"typeName":"image-generator","role":"Illustrator","initialPrompt":"Draw a bird","inherit":true}],"results":[{"conversationId":"child-fixture","logAbsoluteUri":"file:///fixture/child/transcript.jsonl"}]}}]}`
const syntheticManageRaw = `{"cascadeId":"parent-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"manage_task"}},"generic":{"args":{"Action":"status","TaskId":"task-fixture","toolAction":"Checking task","toolSummary":"Task status"},"result":{"result":"Task completed","fullOutputUri":"file:///fixture/task/output.txt","stepRenderInfo":{"title":"Background command execution","titlePrefix":"Checked task","icon":"task_alt"},"metadata":{"task_description":"echo fixture","task_id":"task-fixture","tool_name":"run_command"}}}}]}`
const syntheticSendRaw = `{"cascadeId":"child-fixture","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"send_message"}},"generic":{"args":{"Message":"Image ready","Recipient":"parent-fixture","toolAction":"Sending image","toolSummary":"Image result"},"result":{"result":"Message sent","fullOutputUri":"file:///fixture/message/output.txt","stepRenderInfo":{"title":"Parent Agent","titlePrefix":"Messaged","markdown":"> Image ready","icon":"send"}}}}]}`
const syntheticImageEnvelope = `{"cascadeId":"child-fixture","metadata":{"parentConversationId":"parent-fixture","rootConversationId":"root-fixture","subagentSpec":{"typeName":"image-generator","role":"Illustrator"},"staticConfig":{"agentConfig":{"cascadeConfig":{"plannerConfig":{"toolConfig":{"generateImage":{"saveToParentArtifactDirectory":true}}}}}},"agentScript":{"name":"image-generator","config":{"agentConfig":{"cascadeConfig":{"plannerConfig":{"toolConfig":{"generateImage":{"saveToParentArtifactDirectory":true}}}}}}}},"executorMetadatas":[{"cascadeConfig":{"plannerConfig":{"toolConfig":{"generateImage":{"saveToParentArtifactDirectory":true}}}}}],"generatorMetadata":[{"chatModel":{"customMetadata":{"parent_cascade_id":"parent-fixture","root_cascade_id":"root-fixture"}}}],`
const syntheticImageRaw = syntheticImageEnvelope + `"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"generate_image"},"completedAt":"2026-01-01T00:00:02Z"},"generic":{"args":{"Prompt":"Draw a bird","ImageName":"bird","AspectRatio":"1:1","toolAction":"Drawing bird","toolSummary":"Bird illustration"},"result":{"result":"Image saved","fullOutputUri":"file:///fixture/child/image-output.txt","media":[{"mimeType":"image/jpeg","uri":"file:///fixture/parent/bird.jpg"}],"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.GenerateImage","prompt":"Draw a bird","aspectRatio":"1:1","imageUri":"file:///fixture/parent/bird.jpg","imageNumBytes":1234,"modelName":"fixture-image-model","imageName":"bird"},"stepRenderInfo":{"titlePrefix":"Generated","icon":"image","aggregationKey":"images","titleParts":[{"text":"bird"}],"contentBlocks":[{"media":{"uri":"file:///fixture/parent/bird.jpg","mimeType":"image/jpeg","alt":"bird"}}]}}}}]}`
const syntheticImagePresentation = syntheticImageEnvelope + `"steps":[{"type":"CORTEX_STEP_TYPE_GENERATE_IMAGE","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"generate_image"},"completedAt":"2026-01-01T00:00:02Z"},"generateImage":{"prompt":"Draw a bird","imageName":"bird","aspectRatio":"1:1","modelName":"fixture-image-model","generatedMedia":{"mimeType":"image/jpeg","uri":"file:///fixture/parent/bird.jpg"}}}]}`

func trajectory1216(t *testing.T, raw string) proto.Message {
	t.Helper()
	m, err := newMessage("gemini_coder.Trajectory")
	if err != nil {
		t.Fatal(err)
	}
	if err = (protojson.UnmarshalOptions{Resolver: schemaTypes}).Unmarshal([]byte(raw), m); err != nil {
		t.Fatal(err)
	}
	return m
}

func snapshot1216(t *testing.T, raw string) *Snapshot {
	t.Helper()
	s, err := finishSnapshot(trajectory1216(t, raw))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func replace1216(t *testing.T, raw, before, after string) string {
	t.Helper()
	if !strings.Contains(raw, before) || before == after {
		t.Fatalf("invalid fixture mutation: %q", before)
	}
	return strings.ReplaceAll(raw, before, after)
}

func require1216(t *testing.T, s *Snapshot, raw, presentation string, want State) {
	t.Helper()
	a, result := s.VerifyResponses([]byte(raw), []byte(presentation))
	if result.State != want {
		t.Fatalf("got %+v, want %s", result, want)
	}
	if want == Verified {
		if result := s.CheckAttestation([]byte(presentation), a); result.State != Verified {
			t.Fatalf("attestation: %+v", result)
		}
	} else if a != nil {
		t.Fatal("unverified responses minted an attestation")
	}
}

func TestProjection1216QualifiedShapes(t *testing.T) {
	for name, pair := range map[string][2]string{
		"invoke subagent": {syntheticInvokeRaw, syntheticInvokePresentation},
		"manage task":     {syntheticManageRaw, syntheticManageRaw},
		"send message":    {syntheticSendRaw, syntheticSendRaw},
		"generate image":  {syntheticImageRaw, syntheticImagePresentation},
	} {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, pair[0])
			require1216(t, s, pair[0], pair[1], Verified)
		})
	}
}

func TestProjection1216PassThroughChanges(t *testing.T) {
	for name, raw := range map[string]string{"manage task": syntheticManageRaw, "send message": syntheticSendRaw} {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, raw)
			for field, change := range map[string][2]string{
				"result text":      {`"result":{"result":"`, `"result":{"result":"stale `},
				"output URI":       {`file:///fixture/`, `file:///stale/`},
				"render title":     {`"title":"`, `"title":"stale `},
				"added media":      {`"result":{"result":`, `"result":{"media":[{"mimeType":"image/png","uri":"file:///fixture/extra.png"}],"result":`},
				"tool identity":    {`"name":"`, `"name":"unknown_`},
				"argument content": {`"toolSummary":"`, `"toolSummary":"stale `},
				"removed full URI": {`"fullOutputUri":`, `"discardedFullOutputUri":`},
			} {
				t.Run(field, func(t *testing.T) {
					changed := replace1216(t, raw, change[0], change[1])
					require1216(t, s, raw, changed, Incomplete)
					require1216(t, s, changed, raw, Incomplete)
				})
			}
		})
	}
}

func TestProjection1216ImageAndLineageChanges(t *testing.T) {
	s := snapshot1216(t, syntheticImageRaw)
	for field, change := range map[string][2]string{
		"prompt":             {`Draw a bird`, `Draw a flower`},
		"image name":         {`"imageName":"bird"`, `"imageName":"flower"`},
		"aspect ratio":       {`"aspectRatio":"1:1"`, `"aspectRatio":"4:3"`},
		"model":              {`fixture-image-model`, `different-model`},
		"artifact URI":       {`file:///fixture/parent/bird.jpg`, `file:///fixture/child/bird.jpg`},
		"media MIME":         {`image/jpeg`, `image/png`},
		"parent lineage":     {`"parentConversationId":"parent-fixture"`, `"parentConversationId":"other-parent"`},
		"root lineage":       {`"rootConversationId":"root-fixture"`, `"rootConversationId":"other-root"`},
		"subagent identity":  {`"role":"Illustrator"`, `"role":"Reviewer"`},
		"artifact ownership": {`"saveToParentArtifactDirectory":true`, `"saveToParentArtifactDirectory":false`},
		"generator lineage":  {`"parent_cascade_id":"parent-fixture"`, `"parent_cascade_id":"other-parent"`},
		"outer metadata":     {`2026-01-01T00:00:02Z`, `2026-01-01T00:00:03Z`},
	} {
		t.Run(field, func(t *testing.T) {
			presentation := replace1216(t, syntheticImagePresentation, change[0], change[1])
			require1216(t, s, syntheticImageRaw, presentation, Incomplete)
			raw := replace1216(t, syntheticImageRaw, change[0], change[1])
			require1216(t, s, raw, syntheticImagePresentation, Incomplete)
		})
	}
	for field, change := range map[string][2]string{
		"image size":    {`"imageNumBytes":1234`, `"imageNumBytes":4321`},
		"result text":   {`Image saved`, `Image unavailable`},
		"wrapper media": {`"media":[{"mimeType":"image/jpeg"`, `"media":[{"mimeType":"image/png"`},
		"render media":  {`"alt":"bird"`, `"alt":"flower"`},
		"full output":   {`file:///fixture/child/image-output.txt`, `file:///fixture/child/stale.txt`},
	} {
		t.Run(field, func(t *testing.T) {
			raw := replace1216(t, syntheticImageRaw, change[0], change[1])
			require1216(t, s, raw, syntheticImagePresentation, Incomplete)
		})
	}
}

func TestProjection1216InvokeChanges(t *testing.T) {
	s := snapshot1216(t, syntheticInvokeRaw)
	for field, change := range map[string][2]string{
		"child identity": {`child-fixture`, `other-child`},
		"child log":      {`file:///fixture/child/transcript.jsonl`, `file:///fixture/other/transcript.jsonl`},
		"subagent role":  {`Illustrator`, `Reviewer`},
		"task details":   {`task-fixture`, `other-task`},
		"inner metadata": {`2026-01-01T00:00:01Z`, `2026-01-01T00:00:03Z`},
	} {
		t.Run(field, func(t *testing.T) {
			require1216(t, s, syntheticInvokeRaw, replace1216(t, syntheticInvokePresentation, change[0], change[1]), Incomplete)
		})
	}
	for field, change := range map[string][2]string{
		"wrapper metadata": {`2026-01-01T00:00:02Z`, `2026-01-01T00:00:04Z`},
		"wrapper media":    {`"result":{"result":`, `"result":{"media":[{"mimeType":"image/jpeg","uri":"file:///fixture/extra.jpg"}],"result":`},
	} {
		t.Run(field, func(t *testing.T) {
			require1216(t, s, replace1216(t, syntheticInvokeRaw, change[0], change[1]), syntheticInvokePresentation, Incomplete)
		})
	}
}

func TestProjection1216UnsupportedShapes(t *testing.T) {
	for _, tc := range []struct {
		name, source, before, after string
	}{
		{"unknown passthrough tool", syntheticSendRaw, `"name":"send_message"`, `"name":"future_tool"`},
		{"missing passthrough identity", syntheticSendRaw, `"name":"send_message"`, ``},
		{"unknown send argument", syntheticSendRaw, `"args":{`, `"args":{"Future":"new",`},
		{"unrelated known send argument", syntheticSendRaw, `"args":{`, `"args":{"CommandLine":"echo unsafe",`},
		{"unknown task argument", syntheticManageRaw, `"args":{`, `"args":{"Future":"new",`},
		{"unknown task metadata", syntheticManageRaw, `"task_description":`, `"future_metadata":`},
		{"unknown invoke argument", syntheticInvokeRaw, `"args":{`, `"args":{"Future":"new",`},
		{"wrong invoke identity", syntheticInvokeRaw, `"name":"invoke_subagent"`, `"name":"run_command"`},
		{"conflicting invoke task", syntheticInvokeRaw, `"invokeSubagent":`, `"taskDetails":{"id":"conflicting-task"},"invokeSubagent":`},
		{"unsupported embedded native", syntheticInvokeRaw, `CORTEX_STEP_TYPE_INVOKE_SUBAGENT`, `CORTEX_STEP_TYPE_USER_INPUT`},
		{"unknown image argument", syntheticImageRaw, `"args":{`, `"args":{"Future":"new",`},
		{"unrelated known image argument", syntheticImageRaw, `"args":{`, `"args":{"CommandLine":"echo unsafe",`},
		{"wrong image tool identity", syntheticImageRaw, `"name":"generate_image"`, `"name":"run_command"`},
		{"unqualified image paths", syntheticImageRaw, `"imageNumBytes":1234`, `"imageNumBytes":1234,"imagePaths":["/fixture/reference.png"]`},
		{"missing image media", syntheticImageRaw, `"media":[{"mimeType":"image/jpeg","uri":"file:///fixture/parent/bird.jpg"}],`, ``},
		{"multiple image media", syntheticImageRaw, `"media":[{"mimeType":"image/jpeg","uri":"file:///fixture/parent/bird.jpg"}]`, `"media":[{"mimeType":"image/jpeg","uri":"file:///fixture/parent/bird.jpg"},{"mimeType":"image/jpeg","uri":"file:///fixture/parent/extra.jpg"}]`},
		{"conflicting image URI", syntheticImageRaw, `"imageUri":"file:///fixture/parent/bird.jpg"`, `"imageUri":"file:///fixture/child/other.jpg"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := replace1216(t, tc.source, tc.before, tc.after)
			require1216(t, snapshot1216(t, raw), raw, raw, Unknown)
		})
	}
	for name, source := range map[string]string{"invoke": syntheticInvokeRaw, "image": syntheticImageRaw} {
		t.Run("known but unsupported "+name+" Any type", func(t *testing.T) {
			m := trajectory1216(t, source)
			payload := message1216(step1216(m), "generic", "result", "payload")
			fields := payload.Descriptor().Fields()
			payload.Set(fields.ByName("type_url"), protoreflect.ValueOfString("type.googleapis.com/gemini_coder.Trajectory"))
			payload.Set(fields.ByName("value"), protoreflect.ValueOfBytes(nil))
			s, err := finishSnapshot(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := project(s.trajectory); err == nil {
				t.Fatal("unqualified Any type accepted merely because its descriptor exists")
			}
		})
		t.Run("missing "+name+" payload", func(t *testing.T) {
			m := trajectory1216(t, source)
			result := message1216(step1216(m), "generic", "result")
			result.Clear(result.Descriptor().Fields().ByName("payload"))
			s, err := finishSnapshot(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := project(s.trajectory); err == nil {
				t.Fatal("missing required payload accepted")
			}
		})
	}
}

func step1216(m proto.Message) protoreflect.Message {
	r := m.ProtoReflect()
	return r.Get(r.Descriptor().Fields().ByName("steps")).List().Get(0).Message()
}

func message1216(m protoreflect.Message, path ...protoreflect.Name) protoreflect.Message {
	for _, name := range path {
		m = m.Get(m.Descriptor().Fields().ByName(name)).Message()
	}
	return m
}

func TestProjection1216UnknownPersistedContent(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		path         []protoreflect.Name
	}{
		{"task render", syntheticManageRaw, []protoreflect.Name{"generic", "result", "step_render_info"}},
		{"message render", syntheticSendRaw, []protoreflect.Name{"generic", "result", "step_render_info"}},
		{"image render", syntheticImageRaw, []protoreflect.Name{"generic", "result", "step_render_info"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := trajectory1216(t, tc.source)
			nested := message1216(step1216(m), tc.path...)
			nested.SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 50000, protowire.VarintType), 1))
			if _, err := finishSnapshot(m); err == nil {
				t.Fatal("unknown nested persisted field accepted")
			}
		})
	}
	t.Run("nested subagent specification", func(t *testing.T) {
		m := trajectory1216(t, syntheticInvokeRaw)
		payload := message1216(step1216(m), "generic", "result", "payload")
		valueField := payload.Descriptor().Fields().ByName("value")
		native, err := decode("gemini_coder.Step", payload.Get(valueField).Bytes())
		if err != nil {
			t.Fatal(err)
		}
		invoke := message1216(native.ProtoReflect(), "invoke_subagent")
		spec := invoke.Get(invoke.Descriptor().Fields().ByName("subagents")).List().Get(0).Message()
		spec.SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 50000, protowire.VarintType), 1))
		value, err := proto.Marshal(native)
		if err != nil {
			t.Fatal(err)
		}
		payload.Set(valueField, protoreflect.ValueOfBytes(value))
		if _, err := finishSnapshot(m); err == nil {
			t.Fatal("unknown nested subagent field accepted")
		}
	})
	for name, source := range map[string]string{"invoke": syntheticInvokeRaw, "image": syntheticImageRaw} {
		for _, mutation := range []string{"unknown field", "unknown type", "invalid payload"} {
			t.Run(name+" "+mutation, func(t *testing.T) {
				m := trajectory1216(t, source)
				payload := message1216(step1216(m), "generic", "result", "payload")
				f := payload.Descriptor().Fields()
				switch mutation {
				case "unknown field":
					value := bytes.Clone(payload.Get(f.ByName("value")).Bytes())
					value = protowire.AppendVarint(protowire.AppendTag(value, 50000, protowire.VarintType), 1)
					payload.Set(f.ByName("value"), protoreflect.ValueOfBytes(value))
				case "unknown type":
					payload.Set(f.ByName("type_url"), protoreflect.ValueOfString("type.googleapis.com/fixture.Unsupported"))
				case "invalid payload":
					payload.Set(f.ByName("value"), protoreflect.ValueOfBytes([]byte{0xff}))
				}
				if _, err := finishSnapshot(m); err == nil {
					t.Fatal("unsupported persisted Any accepted")
				}
			})
		}
	}
}
