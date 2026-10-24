package completeness

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCommandWaitPresentations147(t *testing.T) {
	raw, old := tool143Fixtures[0].pair()
	current := strings.Replace(old, `"runCommand":{`, `"runCommand":{"waitMsBeforeAsync":"5000",`, 1)
	s := snapshot1216(t, raw)
	for _, shown := range []string{old, current} {
		require1216(t, s, raw, shown, Verified)
		a, r := s.VerifyResponses([]byte(raw), []byte(shown))
		if r.State != Verified {
			t.Fatal(r)
		}
		if r = s.CheckAttestation([]byte(shown), a); r.State != Verified {
			t.Fatal(r)
		}
		a.Profile = strings.TrimSuffix(Profile, "-replay147")
		if r = s.CheckAttestation([]byte(shown), a); r.State != Unknown {
			t.Fatal("old profile accepted", r)
		}
	}
	for _, value := range []string{"5001", "10000", "0", "18446744073709551615"} {
		require1216(t, s, raw, strings.Replace(current, `"waitMsBeforeAsync":"5000"`, `"waitMsBeforeAsync":"`+value+`"`, 1), Incomplete)
	}
	// Unknown differently-cased keys must not confuse presence detection.
	zero := strings.Replace(current, `"waitMsBeforeAsync":"5000"`, `"waitMsBeforeAsync":"0"`, 1)
	for _, extra := range []string{`"Steps":true`, `"Steps":null`, `"STEPS":[]`} {
		bad := strings.Replace(zero, `{`, `{`+extra+`,`, 1)
		require1216(t, s, raw, bad, Incomplete)
		valid := strings.Replace(current, `{`, `{`+extra+`,`, 1)
		require1216(t, s, raw, valid, Verified)
	}
	require1216(t, s, strings.Replace(raw, `"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"10000"`, 1), current, Incomplete)
	exported, err := s.OfflineJSON()
	if err != nil {
		t.Fatal(err)
	}
	if r := s.ComparePresentation(exported); r.State != Verified {
		t.Fatal(r)
	}
	if !strings.Contains(string(exported), `"waitMsBeforeAsync": "5000"`) {
		t.Fatal("offline export omitted stored wait")
	}
	// Both individual forms are observed, but a mixed trajectory is not.
	duplicate := func(a, b string) string {
		var x, y map[string]json.RawMessage
		if err := json.Unmarshal([]byte(a), &x); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(b), &y); err != nil {
			t.Fatal(err)
		}
		var xs, ys []json.RawMessage
		if err := json.Unmarshal(x["steps"], &xs); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(y["steps"], &ys); err != nil {
			t.Fatal(err)
		}
		x["steps"], _ = json.Marshal(append(xs, ys...))
		data, _ := json.Marshal(x)
		return string(data)
	}
	two := duplicate(raw, raw)
	snap := snapshot1216(t, two)
	require1216(t, snap, two, duplicate(old, old), Verified)
	require1216(t, snap, two, duplicate(current, current), Verified)
	require1216(t, snap, two, duplicate(old, current), Incomplete)
	// Invalid numeric spellings cannot manufacture an added wait, overflow or panic.
	for _, value := range []string{"-1", "+5000", "05000", "5e3", "5000.0", "18446744073709551616", "bad"} {
		bad := strings.Replace(raw, `"WaitMsBeforeAsync":"5000"`, `"WaitMsBeforeAsync":"`+value+`"`, 1)
		require1216(t, snapshot1216(t, bad), bad, current, Incomplete)
	}
}

func imageReadPair147(mime string) (string, string) {
	common := `"status":"CORTEX_STEP_STATUS_DONE","metadata":{"toolCall":{"name":"read_file"}}`
	media := `{"mimeType":"` + mime + `","inlineData":"aW1hZ2U="}`
	content := `"Showing binary file of type ` + mime + `"`
	envelope := func(step string) string {
		return `{"cascadeId":"fixture-parent","metadata":{},"steps":[{` + step + `}]}`
	}
	raw := envelope(`"type":"CORTEX_STEP_TYPE_GENERIC",` + common + `,"generic":{"args":{"Path":"/fixture/image"},"result":{"result":` + content + `,"media":[` + media + `],"stepRenderInfo":{"titlePrefix":"Analyzed"},"payload":{"@type":"type.googleapis.com/exa.cortex.core.tools.ReadFile","path":"/fixture/image","fileNumBytes":5}}}`)
	shown := envelope(`"type":"CORTEX_STEP_TYPE_VIEW_FILE",` + common + `,"viewFile":{"absolutePathUri":"file:///fixture/image","numBytes":5,"content":` + content + `,"mediaData":` + media + `}`)
	return raw, shown
}

func TestImageRead147(t *testing.T) {
	for _, mime := range []string{"image/jpeg", "image/png", "image/webp"} {
		raw, shown := imageReadPair147(mime)
		s := snapshot1216(t, raw)
		require1216(t, s, raw, shown, Verified)
		a, r := s.VerifyResponses([]byte(raw), []byte(shown))
		if r.State != Verified {
			t.Fatal(r)
		}
		if r = s.CheckAttestation([]byte(shown), a); r.State != Verified {
			t.Fatal(r)
		}
		for _, mutation := range [][2]string{{`aW1hZ2U=`, `YWx0ZXI=`}, {`"numBytes":5`, `"numBytes":6`}, {`"mediaData":`, `"ignoredMedia":`}, {`file:///fixture/image`, `file:///fixture/other`}, {`"content":`, `"isSkillFile":true,"content":`}} {
			require1216(t, s, raw, strings.Replace(shown, mutation[0], mutation[1], 1), Incomplete)
		}
		require1216(t, s, strings.Replace(raw, "Analyzed", "Other", 1), shown, Incomplete)
	}
	raw, shown := imageReadPair147("image/png")
	for _, mutation := range [][2]string{
		{`"Path":"/fixture/image"`, `"Path":"/fixture/other"`},
		{`"args":{`, `"args":{"StartLine":"1",`},
		{`"fileNumBytes":5`, `"fileNumBytes":5,"startLine":1`},
		{`"inlineData":"aW1hZ2U="`, `"uri":"file:///fixture/image"`},
		{`"inlineData":"aW1hZ2U="`, `"inlineData":""`},
		{`"media":[`, `"media":[{},`},
		{`"result":{"result":`, `"result":{"fullOutputUri":"file:///fixture/output","result":`},
		{`CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_ERROR`},
		{`"generic":`, `"error":{"shortError":"failed"},"generic":`},
		{`"generic":`, `"permissions":{},"generic":`},
	} {
		bad := strings.Replace(raw, mutation[0], mutation[1], 1)
		require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
	}
	bad, other := imageReadPair147("application/pdf")
	require1216(t, snapshot1216(t, bad), bad, other, Unknown)
}
