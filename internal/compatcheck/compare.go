package compatcheck

import (
	"fmt"
	"regexp"
	"strings"
)

var ulidRe = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

// SwapGateOK reports whether the mcp-integration-lab smoke triplet holds
// plus GET /healthz 200. LabMail additionally requires relay 403.
func SwapGateOK(r Report, requireRelay403 bool) []string {
	var errs []string
	if r.UnauthStatus != 401 {
		errs = append(errs, fmt.Sprintf("%s GET /email unauth status=%d want 401", r.Name, r.UnauthStatus))
	}
	if r.HealthzStatus != 200 {
		errs = append(errs, fmt.Sprintf("%s GET /healthz status=%d want 200", r.Name, r.HealthzStatus))
	}
	if r.ListStatus != 200 {
		errs = append(errs, fmt.Sprintf("%s GET /email basic status=%d want 200", r.Name, r.ListStatus))
	}
	if !r.ListIsArray {
		errs = append(errs, fmt.Sprintf("%s GET /email is not a JSON array", r.Name))
	}
	if !r.ListSubjectHit {
		errs = append(errs, fmt.Sprintf("%s GET /email missing subject %q", r.Name, r.Subject))
	}
	if requireRelay403 && r.RelayStatus != 403 {
		errs = append(errs, fmt.Sprintf("%s POST /email/:id/relay status=%d want 403", r.Name, r.RelayStatus))
	}
	return errs
}

// DocumentedLabMailDeltas checks LabMail-only deltas from docs/12-maildev-compat.md.
func DocumentedLabMailDeltas(r Report) []string {
	var errs []string
	id := str(r.ListItem["id"])
	if id != "" && !ulidRe.MatchString(id) {
		errs = append(errs, fmt.Sprintf("%s id %q is not a ULID", r.Name, id))
	}
	if r.ListItem != nil {
		if str(r.ListItem["text"]) != "" || str(r.ListItem["html"]) != "" {
			errs = append(errs, fmt.Sprintf("%s list leaked bodies text=%q html=%q", r.Name, r.ListItem["text"], r.ListItem["html"]))
		}
		if _, ok := r.ListItem["stream"]; ok {
			errs = append(errs, r.Name+" list leaked stream")
		}
	}
	if atts, ok := r.GetItem["attachments"].([]any); ok {
		for _, raw := range atts {
			att, _ := raw.(map[string]any)
			if att == nil {
				continue
			}
			if _, ok := att["stream"]; ok {
				errs = append(errs, r.Name+" attachment leaked stream")
			}
			sum := str(att["checksum"])
			if sum != "" && len(sum) != 64 {
				errs = append(errs, fmt.Sprintf("%s attachment checksum %q is not sha256 hex", r.Name, sum))
			}
		}
	}
	return errs
}

// SharedShapeDiff compares fields that must match across MailDev 2.2.1 and
// LabMail. id, time, checksum, list bodies, envelope, config, extra maildev
// keys (source/size/stream) are documented deltas and are ignored.
func SharedShapeDiff(maildev, labmail Report) []string {
	var errs []string
	if maildev.ListItem == nil || labmail.ListItem == nil {
		return append(errs, "missing list item for shared-shape compare")
	}
	errs = append(errs, fieldEq("subject", maildev.ListItem["subject"], labmail.ListItem["subject"])...)
	errs = append(errs, fieldEq("priority", maildev.ListItem["priority"], labmail.ListItem["priority"])...)
	errs = append(errs, addrsEq("from", maildev.ListItem["from"], labmail.ListItem["from"])...)
	errs = append(errs, addrsEq("to", maildev.ListItem["to"], labmail.ListItem["to"])...)
	errs = append(errs, headerEq("from", maildev.ListItem, labmail.ListItem)...)
	errs = append(errs, headerEq("to", maildev.ListItem, labmail.ListItem)...)
	errs = append(errs, headerEq("subject", maildev.ListItem, labmail.ListItem)...)
	if maildev.GetItem != nil && labmail.GetItem != nil {
		errs = append(errs, attNamesEq(maildev.GetItem["attachments"], labmail.GetItem["attachments"])...)
	}
	return errs
}

func fieldEq(name string, a, b any) []string {
	if fmt.Sprint(a) != fmt.Sprint(b) {
		return []string{fmt.Sprintf("%s maildev=%v labmail=%v", name, a, b)}
	}
	return nil
}

func addrsEq(name string, a, b any) []string {
	la := addrList(a)
	lb := addrList(b)
	if len(la) != len(lb) {
		return []string{fmt.Sprintf("%s len maildev=%d labmail=%d", name, len(la), len(lb))}
	}
	var errs []string
	for i := range la {
		if la[i].Address != lb[i].Address || la[i].Name != lb[i].Name {
			errs = append(errs, fmt.Sprintf("%s[%d] maildev=%+v labmail=%+v", name, i, la[i], lb[i]))
		}
	}
	return errs
}

type addr struct {
	Address string
	Name    string
}

func addrList(v any) []addr {
	arr, _ := v.([]any)
	out := make([]addr, 0, len(arr))
	for _, el := range arr {
		m, _ := el.(map[string]any)
		if m == nil {
			continue
		}
		out = append(out, addr{Address: str(m["address"]), Name: str(m["name"])})
	}
	return out
}

func headerEq(key string, a, b map[string]any) []string {
	ha := headers(a)
	hb := headers(b)
	// Keys must be lowercased on both. Values for the smoke fields must match
	// after collapsing whitespace; maildev and LabMail both keep the raw header.
	ka, oka := ha[key]
	kb, okb := hb[key]
	if !oka || !okb {
		return []string{fmt.Sprintf("headers.%s present maildev=%v labmail=%v", key, oka, okb)}
	}
	if strings.TrimSpace(ka) != strings.TrimSpace(kb) {
		return []string{fmt.Sprintf("headers.%s maildev=%q labmail=%q", key, ka, kb)}
	}
	return nil
}

func headers(item map[string]any) map[string]string {
	raw, _ := item["headers"].(map[string]any)
	out := map[string]string{}
	for k, v := range raw {
		out[strings.ToLower(k)] = fmt.Sprint(v)
	}
	return out
}

func attNamesEq(a, b any) []string {
	na := attNames(a)
	nb := attNames(b)
	if strings.Join(na, ",") != strings.Join(nb, ",") {
		return []string{fmt.Sprintf("attachment fileName maildev=%v labmail=%v", na, nb)}
	}
	return nil
}

func attNames(v any) []string {
	arr, _ := v.([]any)
	var out []string
	for _, el := range arr {
		m, _ := el.(map[string]any)
		if m == nil {
			continue
		}
		if n := str(m["fileName"]); n != "" {
			out = append(out, n)
		}
	}
	return out
}
