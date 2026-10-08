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
// change operations, then runs CoerceWireTree. A top-level object folds the
// one member whose name matches "operations" in any case (encoding/json
// matches that spelling into the operations field). Two or more such members
// are a duplicate key and neither value is folded. The state member stays
// exact. An array is the operations list itself. A duplicate fold inside an
// operation is one violation and neither key is coerced.
func CoerceWireChange(v any) []domainerr.FieldViolation {
	var folded []domainerr.FieldViolation
	switch x := v.(type) {
	case map[string]any:
		folded = foldOperationsMember(x)
	case []any:
		folded = foldWireKeys(x, "")
	}
	if len(folded) > 0 {
		return folded
	}
	return CoerceWireTree(v)
}

// foldOperationsMember selects every key that matches "operations"
// case-insensitively. One match is folded even when the spelling is not
// "operations". Two or more matches are one duplicate-key violation and
// neither value is folded or coerced.
func foldOperationsMember(x map[string]any) []domainerr.FieldViolation {
	keys := make([]string, 0, len(x))
	for k := range x {
		keys = append(keys, k)
	}
	matches := make([]string, 0, 1)
	for _, k := range keys {
		if strings.EqualFold(k, "operations") {
			matches = append(matches, k)
		}
	}
	switch len(matches) {
	case 0:
		return nil
	case 1:
		ops := x[matches[0]]
		if ops == nil {
			return nil
		}
		return foldWireKeys(ops, "operations")
	default:
		return []domainerr.FieldViolation{{
			Path:    "operations",
			Code:    violationDuplicateKey,
			Message: `duplicate key "operations"`,
		}}
	}
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
