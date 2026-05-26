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
