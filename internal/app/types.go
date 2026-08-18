package app

import (
	"encoding/json"
	"time"

	"github.com/hilather/go-lab-maildev/internal/audit"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// Actor is the caller identity recorded on audit. Authorization lands in SEC-001.
type Actor struct {
	ID        string
	Class     string
	Transport string
}

// ChangeIn is the shared plan/apply envelope.
type ChangeIn struct {
	ExpectedRevision model.Revision
	IdempotencyKey   string
	Reason           string
	Force            bool
	Operations       []model.Operation
}

// ValidateIn validates a candidate document and/or operations.
type ValidateIn struct {
	State      *model.State
	Operations []model.Operation
}

// ResetIn is the privileged bootstrap reread. expectedRevision is not required.
type ResetIn struct {
	Reason string
}

// Plan is the dry-run result of validate/plan (and the body of apply).
type Plan struct {
	PreviousRevision  model.Revision
	CandidateRevision model.Revision
	Drifted           bool
	Diff              []DiffEntry
	Warnings          []Warning
	Operations        []model.Operation
}

// ApplyResult is a committed mutation result.
type ApplyResult struct {
	Plan
	Applied         bool
	Generation      model.Generation
	RuntimeRevision model.Revision
	AuditEventID    string
}

// ExportFormat selects canonical YAML or JSON. Comments are never preserved.
type ExportFormat string

const (
	ExportYAML ExportFormat = "yaml"
	ExportJSON ExportFormat = "json"
)

// Export is canonical desired state plus drift material.
type Export struct {
	Format            ExportFormat
	Body              []byte
	Revision          model.Revision
	BootstrapRevision model.Revision
	Drifted           bool
	HumanDiff         string
}

// StateView is GET /v1/state. Canonical is a copy; mutating it cannot
// affect the live snapshot.
type StateView struct {
	BootstrapRevision model.Revision
	RuntimeRevision   model.Revision
	Generation        model.Generation
	StoreGeneration   uint64
	Drifted           bool
	LoadedAt          time.Time
	MessageCount      int
	StoreBytes        int64
	Canonical         *model.State
}

// Status is the agent-readable process DTO.
type Status struct {
	Ready       bool
	Revisions   model.RevisionStatus
	UnreadCount int
	Epoch       uint64
}

// DefaultWaitTimeout is the native wait default when the caller omits timeout.
const DefaultWaitTimeout = 10 * time.Second

// WaitIn is POST /v1/messages:wait. Timeout 0 means DefaultWaitTimeout;
// both the default and an explicit timeout are capped by store.maxWait.
type WaitIn struct {
	Filter  model.MessageFilter
	Timeout time.Duration
}

// Inbox event types for REST SSE / MCP notify. Adapters must not import store.
const (
	InboxMailReceived = "mail.received"
	InboxMailDeleted  = "mail.deleted"
	InboxStoreWiped   = "store.wiped"
)

// InboxEvent is one inbox membership change.
type InboxEvent struct {
	Type       string
	ID         string
	Subject    string
	Generation uint64
}

// Warning is a bounded, stable-coded note.
type Warning struct {
	Code    string
	Message string
}

// DiffEntry is one canonical-path change. Paths are sorted in plans.
type DiffEntry struct {
	Path   string          `json:"path"`
	Op     string          `json:"op"`
	Before json.RawMessage `json:"before,omitempty"`
	After  json.RawMessage `json:"after,omitempty"`
}

// DeleteIn is a store-generation-optional inbox delete.
type DeleteIn struct {
	ExpectedStoreGeneration *uint64
}

// AuditQuery lists recent in-memory events.
type AuditQuery struct {
	Limit int
}

// AuditList is a newest-first page of the ring.
type AuditList struct {
	Events []AuditEvent
}

// AuditEvent is one mutation or security record.
type AuditEvent = audit.Event

// ExtractResult is POST /v1/messages/{id}:extract.
type ExtractResult struct {
	URLs   []string         `json:"urls"`
	Tokens []ExtractedToken `json:"tokens"`
}

// ExtractedToken is one OTP-like match.
type ExtractedToken struct {
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Context string `json:"context"`
}
