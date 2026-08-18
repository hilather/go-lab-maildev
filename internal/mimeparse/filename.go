package mimeparse

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxFilenameRunes = 200

// SanitizeFilename strips client path components and unsafe runes for download names.
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	var b strings.Builder
	b.Grow(len(name))
	n := 0
	for _, r := range name {
		if r < 32 || r == 127 || r == utf8.RuneError {
			continue
		}
		switch r {
		case '/', ':', '*', '?', '"', '<', '>', '|':
			r = '_'
		}
		if unicode.IsSpace(r) && r != ' ' {
			r = ' '
		}
		b.WriteRune(r)
		n++
		if n >= maxFilenameRunes {
			break
		}
	}
	name = strings.TrimSpace(b.String())
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	return name
}
