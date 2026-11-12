package render

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
)

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

func TestClickableLinkSpecialFileNames(t *testing.T) {
	cases := []struct {
		name      string
		path      string // original absolute path
		wantLabel string
		wantEnc   string // expected encoded path in the target
	}{
		{"ordinary", "/fixture/docs/notes.md", "notes.md", "/fixture/docs/notes.md"},
		{"space and ampersand", "/fixture/docs/Project Notes — Phase 1 & 2.md", "Project Notes — Phase 1 & 2.md", "/fixture/docs/Project%20Notes%20%E2%80%94%20Phase%201%20%26%202.md"},
		{"hash", "/fixture/a#b.md", "a#b.md", "/fixture/a%23b.md"},
		{"question mark", "/fixture/a?b.md", "a?b.md", "/fixture/a%3Fb.md"},
		{"percent", "/fixture/100%.md", "100%.md", "/fixture/100%25.md"},
		{"literal percent 20", "/fixture/a%20b.md", "a%20b.md", "/fixture/a%2520b.md"},
		{"parentheses", "/fixture/a (1).md", "a (1).md", "/fixture/a%20%281%29.md"},
		{"entity text", "/fixture/Notes &amp; Plans.md", `Notes \&amp; Plans.md`, "/fixture/Notes%20%26amp;%20Plans.md"},
		{"emphasis chars", "/fixture/my_file*name_.md", `my\_file\*name\_.md`, "/fixture/my_file%2Aname_.md"},
		{"brackets", "/fixture/a[1].md", `a\[1\].md`, "/fixture/a%5B1%5D.md"},
	}
	for _, tc := range cases {
		for _, prefix := range []string{"file://", ""} {
			t.Run(tc.name+"/"+prefix, func(t *testing.T) {
				got := clickableLink(prefix + tc.path)
				want := "[" + tc.wantLabel + "](" + prefix + tc.wantEnc + ")"
				if got != want {
					t.Fatalf("clickableLink = %q, want %q", got, want)
				}
				target := got[strings.LastIndex(got, "](")+2 : len(got)-1]
				if strings.ContainsAny(target, " ()[]<>#?\"") {
					t.Errorf("target %q has an unencoded delimiter", target)
				}
				decoded, err := url.PathUnescape(strings.TrimPrefix(target, "file://"))
				if err != nil || decoded != tc.path {
					t.Errorf("decoded target = %q (err %v), want %q", decoded, err, tc.path)
				}
			})
		}
	}
}

func TestFenceAndInlineDelimitersHaveBoundedAllocations(t *testing.T) {
	body := strings.Repeat("`", 8192)
	for name, run := range map[string]func(){
		"code fence":  func() { var buf bytes.Buffer; writeFence(&buf, "", body) },
		"inline span": func() { _ = inlineCode(body) },
	} {
		t.Run(name, func(t *testing.T) {
			if allocations := testing.AllocsPerRun(1, run); allocations > 100 {
				t.Fatalf("delimiter selection allocates per backtick: %.0f", allocations)
			}
		})
	}
}
