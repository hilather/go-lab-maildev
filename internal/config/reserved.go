package config

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

// reservedExact are normalized names that are never legal, even as unknown fields.
var reservedExact = map[string]string{
	"forwardto": "configures outbound delivery",
	"mx":        "implies MX lookup / outbound delivery",
	"deliver":   "configures outbound delivery",
}

// reservedPrefixes match after dash/underscore/case normalize.
var reservedPrefixes = []struct {
	prefix string
	why    string
}{
	{"outgoing", "configures an outgoing SMTP relay"},
	{"autorelay", "auto-forwards received mail outward"},
	{"relay", "configures an outbound relay"},
	{"smarthost", "configures an outbound smarthost"},
}

// normalizeKey strips leading dashes, dashes, underscores, and case.
func normalizeKey(k string) string {
	k = strings.TrimLeft(k, "-")
	var b strings.Builder
	b.Grow(len(k))
	for _, r := range k {
		if r == '-' || r == '_' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func reservedReason(normalized string) string {
	if why, ok := reservedExact[normalized]; ok {
		return why
	}
	for _, p := range reservedPrefixes {
		if strings.HasPrefix(normalized, p.prefix) {
			return p.why
		}
	}
	return ""
}

func reservedFields(v any, path string) []domainerr.FieldViolation {
	switch x := v.(type) {
	case map[string]any:
		var vs []domainerr.FieldViolation
		for k, child := range x {
			p := joinPath(path, k)
			if why := reservedReason(normalizeKey(k)); why != "" {
				vs = append(vs, domainerr.FieldViolation{
					Path:    p,
					Code:    violationReservedName,
					Message: fmt.Sprintf("reserved key %q %s — LabMail is receive-only", k, why),
				})
				continue
			}
			vs = append(vs, reservedFields(child, p)...)
		}
		return vs
	case []any:
		var vs []domainerr.FieldViolation
		for i, child := range x {
			vs = append(vs, reservedFields(child, indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}
