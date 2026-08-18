package observability

import "strings"

// forbiddenSet and allowedSet are built once from the catalog tables.
var (
	forbiddenSet = indexStrings(ForbiddenLabels)
	allowedSet   = indexStrings(AllowedLabels)
)

func indexStrings(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, s := range in {
		out[strings.ToLower(s)] = struct{}{}
	}
	return out
}

// ForbiddenLabel reports whether key is a prohibited default label
// (subject, address, client IP, actor, or free-form error text).
func ForbiddenLabel(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	if _, ok := forbiddenSet[k]; ok {
		return true
	}
	if strings.Contains(k, "subject") || strings.Contains(k, "client_ip") ||
		strings.Contains(k, "remote_addr") || strings.Contains(k, "password") {
		return true
	}
	return false
}

// AllowedLabel reports whether key is in the global allowlist.
func AllowedLabel(key string) bool {
	_, ok := allowedSet[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// CheckLabels validates labels against the catalog allowlist for metric.
// Unknown metrics, forbidden keys, and keys not declared on the metric fail.
func CheckLabels(metric string, labels map[string]string) error {
	def, ok := LookupMetric(metric)
	if !ok {
		return labelError("unknown_metric")
	}
	return checkLabelsDef(def, labels)
}

func checkLabelsDef(def MetricDef, labels map[string]string) error {
	allowed := make(map[string]struct{}, len(def.Labels))
	for _, l := range def.Labels {
		allowed[l] = struct{}{}
	}
	for k := range labels {
		if ForbiddenLabel(k) {
			return labelError("forbidden_label")
		}
		if _, ok := allowed[k]; !ok {
			return labelError("unknown_label")
		}
	}
	return nil
}

type labelError string

func (e labelError) Error() string { return string(e) }

// LabelReason is the bounded drop reason for a rejected sample.
func LabelReason(err error) string {
	if err == nil {
		return ""
	}
	if r, ok := err.(labelError); ok {
		return string(r)
	}
	return "invalid"
}

// SMTPSessionResult collapses a session outcome to a bounded label.
func SMTPSessionResult(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "ok", "rejected", "timeout":
		return strings.ToLower(result)
	default:
		return "rejected"
	}
}

// SMTPMessageResult collapses a DATA outcome to a bounded label.
func SMTPMessageResult(result string) string {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "accepted", "too_large", "store_full", "auth", "tls":
		return strings.ToLower(result)
	default:
		return "rejected"
	}
}

// CodeClass collapses an HTTP status to a bounded class.
func CodeClass(status int) string {
	switch {
	case status >= 500:
		return "5xx"
	case status >= 400:
		return "4xx"
	case status >= 300:
		return "3xx"
	case status >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
