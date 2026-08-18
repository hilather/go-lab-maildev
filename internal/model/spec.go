package model

import "time"

const (
	TLSModeOff      = "off"
	TLSModeStartTLS = "starttls"
	TLSModeImplicit = "implicit"

	SMTPAuthNone       = "none"
	SMTPAuthPlainLogin = "plain_login"

	FullPolicyReject      = "reject"
	FullPolicyEvictOldest = "evict_oldest"

	MgmtAuthBearer            = "bearer"
	MgmtAuthBearerAndBasic    = "bearer_and_basic"
	MgmtAuthDevLoopbackUnauth = "dev-loopback-unauth"

	RoleViewer        = "viewer"
	RoleOperator      = "operator"
	RoleAdministrator = "administrator"

	ScopeMailRead      = "mail.read"
	ScopeMailWrite     = "mail.write"
	ScopeMailAdmin     = "mail.admin"
	ScopeMailAuditRead = "mail.audit.read"

	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// ListenersSpec configures the SMTP and management listeners.
type ListenersSpec struct {
	SMTP       SMTPListenerSpec `json:"smtp"`
	Management MgmtListenerSpec `json:"management"`
}

// SMTPListenerSpec is the data-plane listener.
type SMTPListenerSpec struct {
	Address string `json:"address"`
}

// MgmtListenerSpec is the control-plane HTTP listener.
type MgmtListenerSpec struct {
	Address       string      `json:"address"`
	RESTPath      string      `json:"restPath"`
	MCPPath       string      `json:"mcpPath"`
	CompatEnabled bool        `json:"compatEnabled"`
	TLS           ListenerTLS `json:"tls"`
}

// ListenerTLS is optional TLS for the management listener.
type ListenerTLS struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

// SMTPSpec is the receive-only SMTP posture.
type SMTPSpec struct {
	Hostname        string        `json:"hostname"`
	MaxMessageBytes int64         `json:"maxMessageBytes"`
	MaxRecipients   int           `json:"maxRecipients"`
	HideExtensions  []string      `json:"hideExtensions"`
	Auth            SMTPAuthSpec  `json:"auth"`
	TLS             SMTPTLSSpec   `json:"tls"`
	Admission       AdmissionSpec `json:"admission"`
}

// SMTPAuthSpec is optional SMTP AUTH.
type SMTPAuthSpec struct {
	Mode         string `json:"mode"`
	Username     string `json:"username"`
	PasswordFile string `json:"passwordFile"`
}

// SMTPTLSSpec is optional SMTP STARTTLS. mode=implicit is rejected in 1.0.
type SMTPTLSSpec struct {
	Mode     string `json:"mode"`
	Required bool   `json:"required"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

// AdmissionSpec caps concurrent SMTP sessions and in-flight DATA.
type AdmissionSpec struct {
	MaxSessions          int           `json:"maxSessions"`
	MaxSessionsPerIP     int           `json:"maxSessionsPerIP"`
	MaxInFlightData      int           `json:"maxInFlightData"`
	MaxInFlightDataBytes int64         `json:"maxInFlightDataBytes"`
	SessionTimeout       time.Duration `json:"sessionTimeout"`
	CommandIdle          time.Duration `json:"commandIdle"`
	DataIdle             time.Duration `json:"dataIdle"`
}

// StoreSpec is the bounded in-memory inbox.
type StoreSpec struct {
	MaxMessages    int           `json:"maxMessages"`
	MaxBytes       int64         `json:"maxBytes"`
	FullPolicy     string        `json:"fullPolicy"`
	MaxWait        time.Duration `json:"maxWait"`
	SpillDirectory string        `json:"spillDirectory"`
	SpillThreshold int64         `json:"spillThreshold"`
}

// UISpec toggles the embedded SPA. REST/MCP stay up when disabled.
type UISpec struct {
	Enabled bool `json:"enabled"`
}

// ManagementSpec is control-plane authentication and HTTP limits.
type ManagementSpec struct {
	Auth              MgmtAuthSpec `json:"auth"`
	MCP               MCPSpec      `json:"mcp"`
	OriginAllowlist   []string     `json:"originAllowlist"`
	BodyLimit         int64        `json:"bodyLimit"`
	RequestsPerSecond int          `json:"requestsPerSecond"`
	Burst             int          `json:"burst"`
	MaxConcurrent     int          `json:"maxConcurrent"`
}

// MgmtAuthSpec names an auth mode and optional tokens / Basic mapping.
type MgmtAuthSpec struct {
	Mode   string      `json:"mode"`
	Tokens []TokenSpec `json:"tokens"`
	Basic  BasicSpec   `json:"basic"`
}

// TokenSpec is one lab static bearer principal.
type TokenSpec struct {
	ID         string   `json:"id"`
	SecretFile string   `json:"secretFile"`
	Role       string   `json:"role"`
	Scopes     []string `json:"scopes"`
}

// BasicSpec maps HTTP Basic onto a token principal.
type BasicSpec struct {
	Username     string `json:"username"`
	PasswordFile string `json:"passwordFile"`
	TokenRef     string `json:"tokenRef"`
}

// MCPSpec is MCP protocol knobs.
type MCPSpec struct {
	AllowLegacyClients bool `json:"allowLegacyClients"`
}

// ObservabilitySpec is process telemetry configuration.
type ObservabilitySpec struct {
	LogLevel string      `json:"logLevel"`
	Metrics  MetricsSpec `json:"metrics"`
	Audit    AuditSpec   `json:"audit"`
}

// MetricsSpec configures the OpenMetrics listener.
type MetricsSpec struct {
	Listen     string `json:"listen"`
	PublicPath bool   `json:"publicPath"`
}

// AuditSpec sizes the in-process audit ring.
type AuditSpec struct {
	Ring int `json:"ring"`
}

// KnownScope reports whether s is a v1alpha1 mail scope.
func KnownScope(s string) bool {
	switch s {
	case ScopeMailRead, ScopeMailWrite, ScopeMailAdmin, ScopeMailAuditRead:
		return true
	default:
		return false
	}
}

// KnownRole reports whether r is a v1alpha1 token role.
func KnownRole(r string) bool {
	switch r {
	case RoleViewer, RoleOperator, RoleAdministrator:
		return true
	default:
		return false
	}
}
