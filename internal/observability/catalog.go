package observability

import (
	"encoding/json"
	"sort"
)

// CatalogID is the versioned metrics/events document identifier.
// Rename or semantic change of a catalog metric requires a new ID or a
// documented deprecation window.
const CatalogID = "labmail.dev/metrics/v1alpha1"

// CatalogRelPath is the generated catalog artifact.
const CatalogRelPath = "api/metrics/v1alpha1.json"

// Kind is a catalog metric type.
type Kind string

const (
	KindCounter   Kind = "counter"
	KindGauge     Kind = "gauge"
	KindHistogram Kind = "histogram"
)

// Frozen metric names. These are an operational compatibility surface.
const (
	MetricSMTPSessionsTotal   = "labmail_smtp_sessions_total"
	MetricSMTPMessagesTotal   = "labmail_smtp_messages_total"
	MetricSMTPSessionDuration = "labmail_smtp_session_duration_seconds"
	MetricSMTPSessionsActive  = "labmail_smtp_sessions_active"
	MetricStoreMessages       = "labmail_store_messages"
	MetricStoreBytes          = "labmail_store_bytes"
	MetricStoreEvictions      = "labmail_store_evictions_total"
	MetricStoreWaiters        = "labmail_store_waiters"
	MetricHTTPRequestsTotal   = "labmail_http_requests_total"
	MetricHTTPRequestDuration = "labmail_http_request_duration_seconds"
	MetricMCPCallsTotal       = "labmail_mcp_calls_total"
	MetricAuthFailuresTotal   = "labmail_auth_failures_total"
	MetricAuditEventsTotal    = "labmail_audit_events_total"
	MetricTelemetryDropped    = "labmail_telemetry_dropped_total"
)

// Frozen structured-log event names.
const (
	EventSMTPAccepted   = "smtp.accepted"
	EventSMTPRejected   = "smtp.rejected"
	EventSMTPSessionEnd = "smtp.session_end"
	EventStoreInserted  = "store.inserted"
	EventStoreDeleted   = "store.deleted"
	EventStoreWiped     = "store.wiped"
	EventStoreFull      = "store.full"
	EventHTTPRequest    = "http.request"
	EventMCPCall        = "mcp.call"
	EventAuthFailure    = "auth.failure"
	EventAuthSuccess    = "auth.success"
	EventStateReset     = "state.reset"
	EventStateApply     = "state.apply"
)

// AllowedLabels is the default bounded label set. Metric definitions may
// use only a subset. Subjects, addresses, and client IPs are never allowed.
var AllowedLabels = []string{
	"capability",
	"code_class",
	"component",
	"event",
	"reason",
	"result",
	"tool",
}

// ForbiddenLabels must never appear on a catalog metric or recorded sample.
var ForbiddenLabels = []string{
	"actor",
	"actor_id",
	"address",
	"authorization",
	"body",
	"client",
	"client_ip",
	"data",
	"detail",
	"err",
	"error",
	"error_text",
	"from",
	"idempotency",
	"idempotency_key",
	"message",
	"password",
	"peer",
	"raw",
	"remote_addr",
	"source_ip",
	"src",
	"src_ip",
	"subject",
	"to",
}

// MetricDef is one catalog row.
type MetricDef struct {
	Name   string   `json:"name"`
	Kind   Kind     `json:"kind"`
	Help   string   `json:"help"`
	Labels []string `json:"labels"`
	Unit   string   `json:"unit,omitempty"`
}

// EventDef is one stable structured-log event.
type EventDef struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// Document is the versioned catalog artifact.
type Document struct {
	ID              string      `json:"id"`
	Version         string      `json:"version"`
	AllowedLabels   []string    `json:"allowedLabels"`
	ForbiddenLabels []string    `json:"forbiddenLabels"`
	Metrics         []MetricDef `json:"metrics"`
	Events          []EventDef  `json:"events"`
}

// EventFields is the frozen slog JSON field set.
var EventFields = []string{
	"timestamp", "level", "event", "component", "request_id", "message_id",
	"smtp_code", "capability", "result", "error_code", "duration_ms",
	"store_generation",
}

// Metrics returns the frozen first-GA catalog in stable name order.
func Metrics() []MetricDef {
	defs := []MetricDef{
		{Name: MetricSMTPSessionsTotal, Kind: KindCounter, Help: "SMTP sessions that ended.", Labels: []string{"result"}},
		{Name: MetricSMTPMessagesTotal, Kind: KindCounter, Help: "SMTP DATA outcomes.", Labels: []string{"result"}},
		{Name: MetricSMTPSessionDuration, Kind: KindHistogram, Help: "SMTP session lifetime.", Labels: nil, Unit: "seconds"},
		{Name: MetricSMTPSessionsActive, Kind: KindGauge, Help: "Currently admitted SMTP sessions.", Labels: nil},
		{Name: MetricStoreMessages, Kind: KindGauge, Help: "Messages currently in the inbox.", Labels: nil},
		{Name: MetricStoreBytes, Kind: KindGauge, Help: "Resident inbox bytes (raw + decoded).", Labels: nil},
		{Name: MetricStoreEvictions, Kind: KindCounter, Help: "Oldest-message evictions.", Labels: nil},
		{Name: MetricStoreWaiters, Kind: KindGauge, Help: "Blocked Wait callers.", Labels: nil},
		{Name: MetricHTTPRequestsTotal, Kind: KindCounter, Help: "Management HTTP requests.", Labels: []string{"capability", "code_class"}},
		{Name: MetricHTTPRequestDuration, Kind: KindHistogram, Help: "Management HTTP latency.", Labels: []string{"capability"}, Unit: "seconds"},
		{Name: MetricMCPCallsTotal, Kind: KindCounter, Help: "MCP tool invocations.", Labels: []string{"tool", "result"}},
		{Name: MetricAuthFailuresTotal, Kind: KindCounter, Help: "Management authentication failures.", Labels: []string{"reason"}},
		{Name: MetricAuditEventsTotal, Kind: KindCounter, Help: "Audit ring records.", Labels: []string{"event"}},
		{Name: MetricTelemetryDropped, Kind: KindCounter, Help: "Telemetry samples dropped under backpressure or policy.", Labels: []string{"reason"}},
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	for i := range defs {
		defs[i].Labels = append([]string(nil), defs[i].Labels...)
		sort.Strings(defs[i].Labels)
	}
	return defs
}

// Events returns the frozen structured-log event catalog.
func Events() []EventDef {
	names := []string{
		EventSMTPAccepted, EventSMTPRejected, EventSMTPSessionEnd,
		EventStoreInserted, EventStoreDeleted, EventStoreWiped, EventStoreFull,
		EventHTTPRequest, EventMCPCall,
		EventAuthFailure, EventAuthSuccess,
		EventStateReset, EventStateApply,
	}
	out := make([]EventDef, len(names))
	for i, n := range names {
		out[i] = EventDef{Name: n, Fields: append([]string(nil), EventFields...)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// LookupMetric returns the catalog definition for name.
func LookupMetric(name string) (MetricDef, bool) {
	def, ok := metricIndex[name]
	return def, ok
}

var metricIndex = func() map[string]MetricDef {
	defs := Metrics()
	m := make(map[string]MetricDef, len(defs))
	for _, d := range defs {
		m[d.Name] = d
	}
	return m
}()

// Catalog returns the versioned document.
func Catalog() Document {
	return Document{
		ID:              CatalogID,
		Version:         "v1alpha1",
		AllowedLabels:   append([]string(nil), AllowedLabels...),
		ForbiddenLabels: append([]string(nil), ForbiddenLabels...),
		Metrics:         Metrics(),
		Events:          Events(),
	}
}

// RenderCatalog is the generated JSON artifact.
func RenderCatalog() ([]byte, error) {
	b, err := json.MarshalIndent(Catalog(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
