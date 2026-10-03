package render

import "testing"

func TestClickableLink(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{"empty", "", ""},
		{"file uri with filename", "file:///a/b/c.ts", "[c.ts](file:///a/b/c.ts)"},
		{"plain path", "/a/b/c.ts", "[c.ts](/a/b/c.ts)"},
		{"no slash", "thing", "[thing](thing)"},
		{"trailing slash", "file:///a/b/", "[file:///a/b/](file:///a/b/)"},
		{"http uri", "http://example.com/repo", "[repo](http://example.com/repo)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clickableLink(tc.uri); got != tc.want {
				t.Errorf("clickableLink(%q) = %q, want %q", tc.uri, got, tc.want)
			}
		})
	}
}

func TestFormatByteSize(t *testing.T) {
	cases := map[int]string{
		1:                      "1 byte",
		512:                    "512 bytes",
		1023:                   "1023 bytes",
		1024:                   "1024 bytes (1.0 KiB)",
		2516582:                "2516582 bytes (2.4 MiB)",
		3 * 1024 * 1024 * 1024: "3221225472 bytes (3.0 GiB)",
	}
	for n, want := range cases {
		if got := formatByteSize(n); got != want {
			t.Errorf("formatByteSize(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestClickableLinkDataURI(t *testing.T) {
	got := clickableLink("data:image/png;base64,QUJD")
	want := "data URI (`image/png`), embedded, not shown (26 characters)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
