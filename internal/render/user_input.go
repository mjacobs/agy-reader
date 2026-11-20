package render

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

type attachedText struct{ Label, Content string }

// Render persisted text attachments without consulting the current file on disk.
func renderUserInput(buf *bytes.Buffer, input *daemon.UserInput) {
	var items []json.RawMessage
	_ = json.Unmarshal(input.Items, &items)
	var blocks []attachedText
	for _, raw := range items {
		var entry struct {
			Item struct {
				TextBlock *attachedText
			}
		}
		if json.Unmarshal(raw, &entry) == nil && entry.Item.TextBlock != nil {
			blocks = append(blocks, *entry.Item.TextBlock)
		}
	}

	buf.WriteString("### User Request\n")
	writeFence(buf, "", input.UserResponse)
	for _, block := range blocks {
		buf.WriteString("### Attached Text\n")
		if block.Label != "" {
			fmt.Fprintf(buf, "%s\n\n", inlineCode(block.Label))
		}
		writeFence(buf, "", block.Content)
	}
	renderAttachments(buf, "User Attachments", input.Media)
}
