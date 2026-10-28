package completeness

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Synthetic values follow a retained child-to-parent call and a fresh agy
// 1.4.9 call to another conversation. Both keep every source field and add
// only recipient metadata. The expected presentation is independent of project.
const sendMessage149Common = `"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"send_message","argumentsJson":"{\"Recipient\":\"fixture-recipient\",\"Message\":\"Fixture ready.\"}"},"completedAt":"2026-01-01T00:00:02Z"}`
const sendMessage149Result = `"result":"Message sent to \"fixture-recipient\".","fullOutputUri":"file:///fixture/steps/4/output.txt","payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.SendMessage","recipient":"fixture-recipient","recipientName":"Parent Agent","message":"Fixture ready."},"stepRenderInfo":{"titlePrefix":"Messaged","icon":"send","aggregationKey":"actions","titleParts":[{"text":"Parent Agent"}],"contentBlocks":[{"markdown":"> Fixture ready."}]}`
const sendMessage149Raw = `{"cascadeId":"fixture-sender","metadata":{},"steps":[{` + sendMessage149Common + `,"generic":{"args":{"Recipient":"fixture-recipient","Message":"Fixture ready."},"result":{` + sendMessage149Result + `}}}]}`
const sendMessage149Shown = `{"cascadeId":"fixture-sender","metadata":{},"steps":[{` + sendMessage149Common + `,"generic":{"args":{"Recipient":"fixture-recipient","Message":"Fixture ready."},"result":{` + sendMessage149Result + `,"metadata":{"recipient":"fixture-recipient","recipient_name":"Parent Agent"}}}}]}`

func TestSendMessage149(t *testing.T) {
	for _, name := range []string{"Parent Agent", "External Agent"} {
		t.Run(name, func(t *testing.T) {
			raw := strings.ReplaceAll(sendMessage149Raw, "Parent Agent", name)
			shown := strings.ReplaceAll(sendMessage149Shown, "Parent Agent", name)
			s := snapshot1216(t, raw)
			require1216(t, s, raw, shown, Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, raw, string(offline), Verified)
			m := trajectory1216(t, raw)
			before := proto.Clone(m)
			if _, err := project(m); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(before, m) {
				t.Fatal("projection mutated the source")
			}
		})
	}
}

func TestSendMessage149RefusesUnqualifiedShapes(t *testing.T) {
	for name, change := range map[string][2]string{
		"different tool":            {`"name":"send_message"`, `"name":"manage_subagents"`},
		"not done":                  {`"status":"CORTEX_STEP_STATUS_DONE"`, `"status":"CORTEX_STEP_STATUS_ERROR"`},
		"outer error":               {`"generic":`, `"error":{"shortError":"failed","fullError":"failed"},"generic":`},
		"task details":              {`"generic":`, `"taskDetails":{"id":"fixture-task"},"generic":`},
		"permission":                {`"generic":`, `"permissions":{},"generic":`},
		"completed interaction":     {`"generic":`, `"completedInteractions":[{}],"generic":`},
		"requested interaction":     {`"generic":`, `"requestedInteraction":{"permission":{}},"generic":`},
		"extra argument":            {`"args":{`, `"args":{"Future":"new",`},
		"unqualified priority":      {`"args":{`, `"args":{"Priority":"high",`},
		"mismatched call":           {`\"Message\":\"Fixture ready.\"`, `\"Message\":\"Different.\"`},
		"missing call":              {`,"argumentsJson":"{\"Recipient\":\"fixture-recipient\",\"Message\":\"Fixture ready.\"}"`, ``},
		"payload recipient":         {`"recipient":"fixture-recipient"`, `"recipient":"other-recipient"`},
		"payload message":           {`"message":"Fixture ready."`, `"message":"Different."`},
		"missing wrapper recipient": {`"args":{"Recipient":"fixture-recipient",`, `"args":{`},
		"missing wrapper message":   {`"args":{"Recipient":"fixture-recipient","Message":"Fixture ready."}`, `"args":{"Recipient":"fixture-recipient"}`},
		"missing recipient":         {`"recipient":"fixture-recipient",`, ``},
		"missing name":              {`"recipientName":"Parent Agent",`, ``},
		"missing message":           {`,"message":"Fixture ready."`, ``},
		"existing metadata":         {`"result":{"result":`, `"result":{"metadata":{"recipient":"fixture-recipient","recipient_name":"Parent Agent"},"result":`},
		"extra media":               {`"result":{"result":`, `"result":{"media":[{"mimeType":"image/png","inlineData":"AQID"}],"result":`},
		"structured result":         {`"result":{"result":`, `"result":{"resultValue":{"stringValue":"extra"},"result":`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, sendMessage149Raw, change[0], change[1])
			s := snapshot1216(t, bad)
			require1216(t, s, bad, sendMessage149Shown, Unknown)
			if _, err := s.OfflineJSON(); err == nil {
				t.Fatal("offline export accepted an unqualified shape")
			}
			if _, sourceProven := s.compareResponses([]byte(bad), []byte(sendMessage149Shown)); !sourceProven {
				t.Fatal("refusal should follow from source alone")
			}
		})
	}
}

func TestSendMessage149PreservesSourceAndPresentation(t *testing.T) {
	s := snapshot1216(t, sendMessage149Raw)
	for name, change := range map[string][2]string{
		"recipient":      {`"recipient":"fixture-recipient"`, `"recipient":"other-recipient"`},
		"name":           {`"recipientName":"Parent Agent"`, `"recipientName":"Other Agent"`},
		"message":        {`"message":"Fixture ready."`, `"message":"Different."`},
		"result":         {`Message sent to`, `Message not sent to`},
		"output URI":     {`file:///fixture/steps/4/output.txt`, `file:///fixture/other-output.txt`},
		"render data":    {`"markdown":"> Fixture ready."`, `"markdown":"> Different."`},
		"outer metadata": {`2026-01-01T00:00:02Z`, `2026-01-01T00:00:03Z`},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, s, replace1216(t, sendMessage149Raw, change[0], change[1]), sendMessage149Shown, Incomplete)
			require1216(t, s, sendMessage149Raw, replace1216(t, sendMessage149Shown, change[0], change[1]), Incomplete)
		})
	}
	for name, change := range map[string][2]string{
		"wrong metadata recipient": {`"metadata":{"recipient":"fixture-recipient"`, `"metadata":{"recipient":"other-recipient"`},
		"missing metadata":         {`,"metadata":{"recipient":"fixture-recipient","recipient_name":"Parent Agent"}`, ``},
		"wrong name":               {`"recipient_name":"Parent Agent"`, `"recipient_name":"Other Agent"`},
		"extra metadata":           {`"recipient_name":"Parent Agent"`, `"recipient_name":"Parent Agent","future":"value"`},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, s, sendMessage149Raw, replace1216(t, sendMessage149Shown, change[0], change[1]), Incomplete)
		})
	}
}

func TestSendMessage149RejectsUnknownWire(t *testing.T) {
	m := trajectory1216(t, sendMessage149Raw).ProtoReflect()
	step := tool143Get(m, "steps").List().Get(0).Message()
	payload := tool143Get(tool143Get(tool143Get(step, "generic").Message(), "result").Message(), "payload").Message()
	data := append([]byte(nil), tool143Get(payload, "value").Bytes()...)
	data = protowire.AppendTag(data, 9999, protowire.VarintType)
	data = protowire.AppendVarint(data, 1)
	tool143Set(payload, "value", protoreflect.ValueOfBytes(data))
	if _, err := finishSnapshot(m.Interface()); err == nil {
		t.Fatal("unknown SendMessage payload field accepted")
	}
	tool143Set(payload, "type_url", protoreflect.ValueOfString("type.googleapis.com/example.Future"))
	if _, err := finishSnapshot(m.Interface()); err == nil {
		t.Fatal("unknown Any accepted")
	}
}

func TestSendMessage149LegacyAndProfile(t *testing.T) {
	for _, raw := range []string{syntheticSendRaw, argumentsSendRaw} {
		s := snapshot1216(t, raw)
		require1216(t, s, raw, raw, Verified)
		if _, err := s.OfflineJSON(); err != nil {
			t.Fatal(err)
		}
	}
	s := snapshot1216(t, sendMessage149Raw)
	a, r := s.VerifyResponses([]byte(sendMessage149Raw), []byte(sendMessage149Shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = strings.SplitN(Profile, "-sendmessage149", 2)[0]
	if r := s.CheckAttestation([]byte(sendMessage149Shown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}

const invalidSendMessage149Raw = `{"cascadeId":"fixture-sender","metadata":{},"steps":[{"type":"CORTEX_STEP_TYPE_GENERIC","status":"CORTEX_STEP_STATUS_INVALID","metadata":{"toolCall":{"name":"send_message","argumentsJson":"{\"Recipient\":\"fixture-recipient\",\"Message\":\"Fixture ready.\",\"Priority\":\"high\"}"}},"error":{"shortError":"invalid arguments:\n- additional properties 'Priority' not allowed","fullError":"invalid arguments:\n- additional properties 'Priority' not allowed"},"generic":{"args":{"Recipient":"fixture-recipient","Message":"Fixture ready.","Priority":"high"}}}]}`

func TestSendMessage149InvalidPriority(t *testing.T) {
	s := snapshot1216(t, invalidSendMessage149Raw)
	require1216(t, s, invalidSendMessage149Raw, invalidSendMessage149Raw, Verified)
	offline, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	require1216(t, s, invalidSendMessage149Raw, string(offline), Verified)
	for name, change := range map[string][2]string{
		"error":     {`additional properties 'Priority'`, `additional properties 'Future'`},
		"message":   {`Fixture ready.`, `Changed.`},
		"recipient": {`fixture-recipient`, `other-recipient`},
		"priority":  {`high`, `low`},
	} {
		t.Run(name+" preserved", func(t *testing.T) {
			bad := replace1216(t, invalidSendMessage149Raw, change[0], change[1])
			require1216(t, s, invalidSendMessage149Raw, bad, Incomplete)
			require1216(t, s, bad, invalidSendMessage149Raw, Incomplete)
		})
	}
	for name, change := range map[string][2]string{
		"wrong status":      {`CORTEX_STEP_STATUS_INVALID`, `CORTEX_STEP_STATUS_ERROR`},
		"wrong error":       {`additional properties 'Priority'`, `additional properties 'Future'`},
		"wrong full error":  {`"fullError":"invalid arguments:\n- additional properties 'Priority' not allowed"`, `"fullError":"other failure"`},
		"wrong priority":    {`high`, `low`},
		"missing priority":  {`,"Priority":"high"`, ``},
		"missing message":   {`"Message":"Fixture ready.",`, ``},
		"missing recipient": {`"Recipient":"fixture-recipient",`, ``},
		"extra argument":    {`"args":{`, `"args":{"Future":"new",`},
		"mismatched call":   {`\"Message\":\"Fixture ready.\"`, `\"Message\":\"Different.\"`},
		"result":            {`"generic":{"args":`, `"generic":{"result":{"result":"new"},"args":`},
		"task":              {`"generic":`, `"taskDetails":{"id":"fixture-task"},"generic":`},
		"permission":        {`"generic":`, `"permissions":{},"generic":`},
	} {
		t.Run(name+" refused", func(t *testing.T) {
			bad := replace1216(t, invalidSendMessage149Raw, change[0], change[1])
			changed := snapshot1216(t, bad)
			require1216(t, changed, bad, bad, Unknown)
			if _, err := changed.OfflineJSON(); err == nil {
				t.Fatal("unqualified invalid SendMessage accepted")
			}
		})
	}
}

func TestSendMessage149MultilineUnicode(t *testing.T) {
	const message = "Fixture ready.\nUnicode: café ✓"
	for _, pair := range [][2]string{{sendMessage149Raw, sendMessage149Shown}, {invalidSendMessage149Raw, invalidSendMessage149Raw}} {
		update := func(raw string) string {
			t.Helper()
			var top map[string]any
			if err := json.Unmarshal([]byte(raw), &top); err != nil {
				t.Fatal(err)
			}
			step := top["steps"].([]any)[0].(map[string]any)
			generic := step["generic"].(map[string]any)
			generic["args"].(map[string]any)["Message"] = message
			call := step["metadata"].(map[string]any)["toolCall"].(map[string]any)
			var args map[string]any
			if err := json.Unmarshal([]byte(call["argumentsJson"].(string)), &args); err != nil {
				t.Fatal(err)
			}
			args["Message"] = message
			encoded, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			call["argumentsJson"] = string(encoded)
			if result, ok := generic["result"].(map[string]any); ok {
				result["payload"].(map[string]any)["message"] = message
				result["stepRenderInfo"].(map[string]any)["contentBlocks"] = []any{map[string]any{"markdown": "> " + strings.ReplaceAll(message, "\n", "\n> ")}}
			}
			encoded, err = json.Marshal(top)
			if err != nil {
				t.Fatal(err)
			}
			return string(encoded)
		}
		raw, shown := update(pair[0]), update(pair[1])
		s := snapshot1216(t, raw)
		require1216(t, s, raw, shown, Verified)
		offline, err := s.OfflineJSON()
		if err != nil {
			t.Fatal(err)
		}
		require1216(t, s, raw, string(offline), Verified)
		require1216(t, s, raw, strings.ReplaceAll(shown, "café", "cafe"), Incomplete)
	}
}
