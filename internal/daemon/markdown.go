package daemon

import "strings"

// BacktickDelimiter returns a delimiter wider than every backtick run in body,
// and at least minimum bytes wide. One scan and one allocation keep long runs
// linear in time and space, for inline spans as well as fenced blocks.
func BacktickDelimiter(body string, minimum int) string {
	width, run := 1, 0
	for i := 0; i < len(body); i++ {
		if body[i] == '`' {
			run++
			if run >= width {
				width = run + 1
			}
		} else {
			run = 0
		}
	}
	if width < minimum {
		width = minimum
	}
	return strings.Repeat("`", width)
}
