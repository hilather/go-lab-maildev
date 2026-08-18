package config

import "time"

// Materialized v1alpha1 defaults. Zero values on the Go types are not these
// values until Decode/Normalize run.
const (
	DefaultSMTPAddress       = ":1025"
	DefaultMgmtAddress       = ":1080"
	DefaultRESTPath          = "/v1"
	DefaultMCPPath           = "/mcp"
	DefaultSMTPHostname      = "labmail.lab"
	DefaultMaxMessageBytes   = int64(10 << 20)
	DefaultMaxRecipients     = 100
	DefaultMaxSessions       = 256
	DefaultMaxSessionsPerIP  = 32
	DefaultMaxInFlightData   = 8
	DefaultMaxInFlightBytes  = int64(64 << 20)
	DefaultSessionTimeout    = 10 * time.Minute
	DefaultCommandIdle       = 120 * time.Second
	DefaultDataIdle          = 180 * time.Second
	DefaultMaxMessages       = 1000
	DefaultStoreMaxBytes     = int64(256 << 20)
	DefaultStoreMaxWait      = 60 * time.Second
	DefaultSpillThreshold    = int64(256 << 10)
	DefaultBodyLimit         = int64(1 << 20)
	DefaultRequestsPerSecond = 32
	DefaultBurst             = 64
	DefaultMaxConcurrent     = 256
	DefaultAuditRing         = 128
	DefaultMetricsListen     = "127.0.0.1:9090"
	MaxDocumentBytes         = 1 << 20

	violationUnknownField       = "unknown_field"
	violationRequired           = "required"
	violationInvalidValue       = "invalid_value"
	violationReservedName       = "reserved_name"
	violationDuplicateKey       = "duplicate_key"
	violationTooLarge           = "document_too_large"
	violationUnsupportedVersion = "unsupported_version"
	violationUnresolved         = "unresolved_reference"
	violationDuplicateID        = "duplicate_id"
	violationEmptyID            = "empty_id"
)
