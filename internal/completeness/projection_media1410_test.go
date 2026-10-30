package completeness

import (
	"strings"
	"testing"
)

// Synthetic values follow the source, full-RPC and default-RPC shapes captured
// from a native agy 1.4.11 session that read two images: a 3200x2400 PNG, stored
// as a 2000x1500 JPEG under result text that still names a PNG, and a JPEG saved
// with a .png name, whose result text and stored media both name a JPEG.
func convertedImage1410(t *testing.T) (raw, shown string) {
	t.Helper()
	raw, shown = imageReadPair147("image/jpeg")
	for _, change := range [][2]string{
		{`Showing binary file of type image/jpeg`, `Showing binary file of type image/png`},
		{`/fixture/image`, `/fixture/big.png`},
	} {
		raw, shown = replace1216(t, raw, change[0], change[1]), replace1216(t, shown, change[0], change[1])
	}
	// The wrapper and presentation report the original file's size, not the JPEG's.
	return replace1216(t, raw, `"fileNumBytes":5`, `"fileNumBytes":39202418`),
		replace1216(t, shown, `"numBytes":5`, `"numBytes":39202418`)
}

func relabeledJPEG1410(t *testing.T) (raw, shown string) {
	t.Helper()
	raw, shown = imageReadPair147("image/jpeg")
	return replace1216(t, raw, `/fixture/image`, `/fixture/shot.png`), replace1216(t, shown, `/fixture/image`, `/fixture/shot.png`)
}

func TestMedia1410Qualified(t *testing.T) {
	convertedRaw, convertedShown := convertedImage1410(t)
	relabeledRaw, relabeledShown := relabeledJPEG1410(t)
	for name, pair := range map[string][2]string{
		"png stored as jpeg": {convertedRaw, convertedShown},
		"jpeg saved as .png": {relabeledRaw, relabeledShown},
	} {
		t.Run(name, func(t *testing.T) {
			s := snapshot1216(t, pair[0])
			require1216(t, s, pair[0], pair[1], Verified)
			offline, err := s.OfflineJSON()
			if err != nil {
				t.Fatal(err)
			}
			require1216(t, s, pair[0], string(offline), Verified)
		})
	}
}

// A changed presentation must not verify, including one that makes the stored
// media agree with the result text the daemon showed.
func TestMedia1410Changes(t *testing.T) {
	raw, shown := convertedImage1410(t)
	s := snapshot1216(t, raw)
	for name, mutation := range map[string][2]string{
		"media type":    {`"mediaData":{"mimeType":"image/jpeg"`, `"mediaData":{"mimeType":"image/png"`},
		"content":       {`"content":"Showing binary file of type image/png"`, `"content":"Showing binary file of type image/jpeg"`},
		"original size": {`"numBytes":39202418`, `"numBytes":5`},
		"media bytes":   {`aW1hZ2U=`, `YWx0ZXI=`},
		"path":          {`file:///fixture/big.png`, `file:///fixture/other.png`},
	} {
		t.Run(name, func(t *testing.T) {
			require1216(t, s, raw, replace1216(t, shown, mutation[0], mutation[1]), Incomplete)
		})
	}
}

// Only the captured pair is qualified: the result text names a PNG and the
// stored media is a JPEG. Every other disagreement stays unsupported.
func TestMedia1410RefusesUnobservedPairs(t *testing.T) {
	for _, tc := range []struct{ name, text, mime string }{
		{"webp stored as jpeg", "image/webp", "image/jpeg"},
		{"jpeg stored as png", "image/jpeg", "image/png"},
		{"png stored as webp", "image/png", "image/webp"},
		{"jpeg stored as webp", "image/jpeg", "image/webp"},
		{"png named by extension", "image/x-png", "image/jpeg"},
		{"unknown type stored as jpeg", "application/pdf", "image/jpeg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, shown := imageReadPair147(tc.mime)
			raw = replace1216(t, raw, `Showing binary file of type `+tc.mime, `Showing binary file of type `+tc.text)
			shown = replace1216(t, shown, `Showing binary file of type `+tc.mime, `Showing binary file of type `+tc.text)
			require1216(t, snapshot1216(t, raw), raw, shown, Unknown)
		})
	}
	// The converted shape keeps the other 1.4.7 requirements.
	raw, shown := convertedImage1410(t)
	for name, mutation := range map[string][2]string{
		"two media":     {`"media":[`, `"media":[{"mimeType":"image/jpeg","inlineData":"aW1hZ2U="},`},
		"empty media":   {`"inlineData":"aW1hZ2U="`, `"inlineData":""`},
		"media by uri":  {`"inlineData":"aW1hZ2U="`, `"uri":"file:///fixture/big.png"`},
		"byte offset":   {`"args":{`, `"args":{"ByteOffset":"0",`},
		"not completed": {`CORTEX_STEP_STATUS_DONE`, `CORTEX_STEP_STATUS_CANCELED`},
	} {
		t.Run(name, func(t *testing.T) {
			bad := replace1216(t, raw, mutation[0], mutation[1])
			require1216(t, snapshot1216(t, bad), bad, shown, Unknown)
		})
	}
}

func TestMedia1410InvalidatesPreviousProfile(t *testing.T) {
	previous := strings.SplitN(Profile, "-media1410", 2)[0]
	if previous == Profile {
		t.Fatal("profile does not name media1410")
	}
	raw, shown := convertedImage1410(t)
	s := snapshot1216(t, raw)
	a, r := s.VerifyResponses([]byte(raw), []byte(shown))
	if r.State != Verified {
		t.Fatal(r)
	}
	a.Profile = previous
	if r := s.CheckAttestation([]byte(shown), a); r.State != Unknown {
		t.Fatalf("previous profile accepted: %+v", r)
	}
}
