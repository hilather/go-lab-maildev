package config

import (
	"strings"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

// CoerceWireTree converts duration and byte-size strings in a decoded JSON
// tree (UseNumber) so encoding/json can populate model types. Keys must
// already be the canonical spelling. Config documents use decodeRaw, not this.
func CoerceWireTree(v any) []domainerr.FieldViolation {
	vs := convertDurations(v, "")
	return append(vs, convertByteSizes(v, "")...)
}

// CoerceWireChange folds case-variant duration and byte-size keys inside
// change operations, then runs CoerceWireTree. A top-level object folds only
// its operations member (the state member stays exact). An array is treated
// as the operations list itself. A duplicate fold is one violation and
// neither key is coerced.
func CoerceWireChange(v any) []domainerr.FieldViolation {
	var folded []domainerr.FieldViolation
	switch x := v.(type) {
	case map[string]any:
		if ops, ok := x["operations"]; ok && ops != nil {
			folded = foldWireKeys(ops, "operations")
		}
	case []any:
		folded = foldWireKeys(x, "")
	}
	if len(folded) > 0 {
		return folded
	}
	return CoerceWireTree(v)
}

// CanonicalDurationField returns the durationFields spelling for name.
func CanonicalDurationField(name string) (string, bool) {
	if durationFields[name] {
		return name, true
	}
	for k := range durationFields {
		if strings.EqualFold(k, name) {
			return k, true
		}
	}
	return "", false
}

// CanonicalSizeField returns the sizeFields spelling for name.
func CanonicalSizeField(name string) (string, bool) {
	if sizeFields[name] {
		return name, true
	}
	for k := range sizeFields {
		if strings.EqualFold(k, name) {
			return k, true
		}
	}
	return "", false
}

func canonicalWireField(name string) (string, bool) {
	if c, ok := CanonicalDurationField(name); ok {
		return c, true
	}
	return CanonicalSizeField(name)
}

// foldWireKeys renames case-variant duration and byte-size keys to the
// canonical spelling. Two keys that fold to one name are left in place and
// reported once. The key slice is snapshotted before mutation.
func foldWireKeys(v any, path string) []domainerr.FieldViolation {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		groups := make(map[string][]string)
		for _, k := range keys {
			canon, ok := canonicalWireField(k)
			if !ok {
				continue
			}
			groups[canon] = append(groups[canon], k)
		}
		var vs []domainerr.FieldViolation
		for canon, ks := range groups {
			if len(ks) > 1 {
				vs = append(vs, domainerr.FieldViolation{
					Path:    joinPath(path, canon),
					Code:    violationInvalidValue,
					Message: "duplicate key",
				})
				continue
			}
			k := ks[0]
			if k == canon {
				continue
			}
			x[canon] = x[k]
			delete(x, k)
		}
		childKeys := make([]string, 0, len(x))
		for k := range x {
			childKeys = append(childKeys, k)
		}
		for _, k := range childKeys {
			vs = append(vs, foldWireKeys(x[k], joinPath(path, k))...)
		}
		return vs
	case []any:
		var vs []domainerr.FieldViolation
		for i, child := range x {
			vs = append(vs, foldWireKeys(child, indexPath(path, i))...)
		}
		return vs
	default:
		return nil
	}
}

// FormatWireTree rewrites duration and byte-size numbers to canonical strings
// for REST JSON responses.
func FormatWireTree(v any) {
	convertDurationNumbersToStrings(v)
	convertByteSizeNumbersToStrings(v)
}
