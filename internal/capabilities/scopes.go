package capabilities

// Frozen first-GA scopes. Role binding lands in SEC-001; adapters must not
// invent synonyms.
const (
	ScopeMailRead      = "mail.read"
	ScopeMailWrite     = "mail.write"
	ScopeMailAdmin     = "mail.admin"
	ScopeMailAuditRead = "mail.audit.read"
)
