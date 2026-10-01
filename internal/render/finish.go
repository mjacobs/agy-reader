package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mjacobs/agy-reader/internal/daemon"
)

// renderFinish writes the final structured result of a FINISH step. The
// arguments_json form is preferred (pretty-printed when it parses); the
// outputString form is shown only when it adds something different.
func renderFinish(buf *bytes.Buffer, f *daemon.Finish) {
	var args string
	if f.Output != nil {
		args = f.Output.ArgumentsJSON
	}
	if args == "" && f.OutputString == "" {
		buf.WriteString("*(No additional details stored for this step type)*\n\n")
		return
	}
	buf.WriteString("### Final Structured Output\n")
	if args != "" {
		pretty, ok := prettyJSONExact(args)
		if !ok {
			pretty = args
		}
		writeFence(buf, "json", pretty)
	}
	if f.OutputString == "" {
		return
	}
	if args != "" {
		if f.OutputString == args {
			return
		}
		if a, ok := prettyJSONExact(args); ok {
			if o, ok := prettyJSONExact(f.OutputString); ok && a == o {
				return
			}
		}
		buf.WriteString("**Output string:**\n")
	}
	writeFence(buf, "", f.OutputString)
}

// prettyJSONExact indents s without converting numbers to float64, so large
// integers in a structured result print exactly as stored.
func prettyJSONExact(s string) (string, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return "", false
	}
	if _, err := dec.Token(); err != io.EOF {
		return "", false // trailing data after the first value, valid JSON or not
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", false
	}
	return string(b), true
}

// writeFence writes body in a code fence long enough that backtick runs inside
// body cannot close it early.
func writeFence(buf *bytes.Buffer, lang, body string) {
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	fmt.Fprintf(buf, "%s%s\n%s\n%s\n\n", fence, lang, body, fence)
}
