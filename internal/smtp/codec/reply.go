package codec

import (
	"fmt"
	"io"
	"strings"
)

// FormatReply renders a single- or multi-line SMTP reply with CRLF.
func FormatReply(code int, lines ...string) string {
	if len(lines) == 0 {
		lines = []string{""}
	}
	var b strings.Builder
	for i, line := range lines {
		if i == len(lines)-1 {
			fmt.Fprintf(&b, "%03d %s\r\n", code, line)
		} else {
			fmt.Fprintf(&b, "%03d-%s\r\n", code, line)
		}
	}
	return b.String()
}

// WriteReply writes FormatReply to w.
func WriteReply(w io.Writer, code int, lines ...string) error {
	if w == nil {
		return fmt.Errorf("smtp: nil writer")
	}
	_, err := io.WriteString(w, FormatReply(code, lines...))
	return err
}

// SplitVerb returns the uppercase command word and the rest of the line.
func SplitVerb(line string) (verb, arg string) {
	line = strings.TrimRight(line, " \t")
	if line == "" {
		return "", ""
	}
	i := strings.IndexAny(line, " \t")
	if i < 0 {
		return strings.ToUpper(line), ""
	}
	return strings.ToUpper(line[:i]), strings.TrimSpace(line[i+1:])
}
