package server

import (
	"errors"
	"strconv"
	"strings"
)

var (
	errSyntax          = errors.New("smtp: bad arguments")
	errMessageTooLarge = errors.New("smtp: message too large")
	errDiscardBudget   = errors.New("smtp: discard budget exceeded")
)

func parsePathArg(arg, keyword string) (path, params string, err error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", errSyntax
	}
	up := strings.ToUpper(arg)
	key := strings.ToUpper(keyword)
	if !strings.HasPrefix(up, key) {
		return "", "", errSyntax
	}
	rest := strings.TrimSpace(arg[len(keyword):])
	return splitAngle(rest)
}

func splitAngle(s string) (path, params string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", errSyntax
	}
	if s[0] != '<' {
		fields := strings.Fields(s)
		if len(fields) == 0 {
			return "", "", errSyntax
		}
		return fields[0], strings.TrimSpace(strings.TrimPrefix(s, fields[0])), nil
	}
	end := strings.IndexByte(s, '>')
	if end < 0 {
		return "", "", errSyntax
	}
	return s[1:end], strings.TrimSpace(s[end+1:]), nil
}

func parseMailParams(params string) (size int64, sizeSet bool, err error) {
	if strings.TrimSpace(params) == "" {
		return 0, false, nil
	}
	for _, p := range strings.Fields(params) {
		k, v, ok := strings.Cut(p, "=")
		switch strings.ToUpper(k) {
		case "SIZE":
			if !ok || v == "" {
				return 0, false, errSyntax
			}
			n, e := strconv.ParseInt(v, 10, 64)
			if e != nil || n < 0 {
				return 0, false, errSyntax
			}
			size, sizeSet = n, true
		default:
			// BODY, SMTPUTF8, and unknown lab parameters are accepted.
		}
	}
	return size, sizeSet, nil
}

func reserveBytes(declared int64, sizeSet bool, max int64) int64 {
	if max <= 0 {
		return 0
	}
	if !sizeSet || declared <= 0 {
		return max
	}
	if declared < max {
		return declared
	}
	return max
}

func hiddenSet(names []string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[strings.ToUpper(strings.TrimSpace(n))] = true
	}
	return out
}
