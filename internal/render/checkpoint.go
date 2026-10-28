package render

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

func renderCheckpoint(buf *bytes.Buffer, cp *daemon.Checkpoint) {
	buf.WriteString("### Checkpoint / Compaction State\n")
	if len(cp.UserRequests) > 0 {
		fmt.Fprintf(buf, "**User Requests:** %s\n", strings.Join(cp.UserRequests, ", "))
	}
	if len(cp.CapturedUserRequests) > 0 {
		buf.WriteString("\n**Captured User Requests:**\n\n")
		for i, request := range cp.CapturedUserRequests {
			fmt.Fprintf(buf, "#### Captured Request %d\n\n", i+1)
			if request.Text != "" {
				writeFence(buf, "", request.Text)
			} else {
				buf.WriteString("*No request text stored.*\n\n")
			}
			for j, media := range request.Media {
				fmt.Fprintf(buf, "**Attachment %d:**\n", j+1)
				renderMedia(buf, media)
				buf.WriteString("\n")
			}
		}
	}
	if cp.SessionSummary != "" {
		fmt.Fprintf(buf, "\n%s\n\n", cp.SessionSummary)
	}
}

func renderMedia(buf *bytes.Buffer, m daemon.Media) {
	for _, field := range []struct{ label, value string }{
		{"Display Name", m.DisplayName}, {"Media Type", m.MimeType},
		{"Description", m.Description}, {"Media ID", m.MediaID},
	} {
		if field.value != "" {
			fmt.Fprintf(buf, "- **%s:** %s\n", field.label, inlineCode(field.value))
		}
	}
	if m.URI != "" {
		fmt.Fprintf(buf, "- **Stored Media:** %s\n", checkpointMediaReference(m.URI))
	}
	if m.Blobref != nil && m.Blobref.BlobID != "" {
		fmt.Fprintf(buf, "- **Blob ID:** %s (reference only)\n", inlineCode(m.Blobref.BlobID))
	}
	if m.StartPage != nil {
		fmt.Fprintf(buf, "- **Start Page:** %d\n", *m.StartPage)
	}
	if m.EndPage != nil {
		fmt.Fprintf(buf, "- **End Page:** %d\n", *m.EndPage)
	}
	if m.InlineData != "" {
		fmt.Fprintf(buf, "- **Inline Data:** embedded, not shown (%d base64 characters)\n", len(m.InlineData))
	}
	if m.Thumbnail != "" {
		fmt.Fprintf(buf, "- **Thumbnail:** embedded, not shown (%d base64 characters)\n", len(m.Thumbnail))
	}
	if m.URI == "" && (m.Blobref == nil || m.Blobref.BlobID == "") {
		buf.WriteString("- *No stored attachment reference.*\n")
	}
}

// Keep references useful without letting a stored URI introduce executable
// schemes, HTML, new Markdown blocks, or an inline binary payload.
func checkpointMediaReference(uri string) string {
	if summary, ok := summariseDataURI(strings.TrimSpace(uri)); ok {
		return inlineCode(summary)
	}
	if strings.IndexFunc(uri, unicode.IsControl) >= 0 {
		return inlineCode(uri)
	}
	var u *url.URL
	if strings.HasPrefix(uri, "/") {
		// Native checkpoints store materialized attachments as absolute POSIX
		// paths. Treat the whole value as a path, including literal # and %.
		u = &url.URL{Scheme: "file", Path: uri}
	} else {
		var err error
		u, err = url.Parse(uri)
		if err != nil {
			return inlineCode(uri)
		}
	}
	switch strings.ToLower(u.Scheme) {
	case "file", "https", "http":
	default:
		return inlineCode(uri)
	}
	if (u.Scheme == "https" || u.Scheme == "http") && u.Host == "" {
		return inlineCode(uri)
	}
	// An inline code label avoids interpreting filenames as HTML or Markdown.
	// Encode delimiters that Markdown could treat as the end of the target.
	target := strings.NewReplacer("(", "%28", ")", "%29", "[", "%5B", "]", "%5D", "<", "%3C", ">", "%3E", "\\", "%5C", " ", "%20", "\"", "%22").Replace(u.String())
	label := uri
	if i := strings.LastIndex(u.Path, "/"); i >= 0 && i+1 < len(u.Path) {
		label = u.Path[i+1:]
	}
	// Brackets delimit links even inside code spans in some Markdown readers.
	label = strings.NewReplacer("[", "&#91;", "]", "&#93;", "<", "&lt;", ">", "&gt;").Replace(label)
	return fmt.Sprintf("[%s](%s)", inlineCode(label), target)
}
