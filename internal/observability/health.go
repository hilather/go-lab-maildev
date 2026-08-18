package observability

// Warning codes are a bounded, stable Status DTO surface.
const (
	WarnSMTPUnbound     = "smtp_unbound"
	WarnStoreDown       = "store_down"
	WarnMgmtUnbound     = "management_unbound"
	WarnListenerUnbound = "listener_unbound"
)

// MaxWarnings caps the Status warning list.
const MaxWarnings = 16

// Warning is one agent-readable operational note.
type Warning struct {
	Code    string
	Message string
}

// Facts are process observations used to evaluate health.
type Facts struct {
	ProcessDown bool
	SMTPBound   bool
	StoreUp     bool
	// MgmtBound is true when the management listener is accepting.
	MgmtBound bool
	// MgmtOff is true when management was explicitly disabled (off/none/-).
	MgmtOff bool
}

// Probe is liveness and readiness plus bounded warnings.
type Probe struct {
	Live     bool
	Ready    bool
	Warnings []Warning
}

// Evaluate implements pack 09 health semantics:
//   - Live: process is serving (not ProcessDown).
//   - Ready: live, SMTP bound, store initialized, and management bound or
//     explicitly off. Ready does not require MCP clients or a non-empty inbox.
func Evaluate(in Facts) Probe {
	p := Probe{Live: !in.ProcessDown}
	mgmtOK := in.MgmtBound || in.MgmtOff
	p.Ready = p.Live && in.SMTPBound && in.StoreUp && mgmtOK

	add := func(code, msg string) {
		if len(p.Warnings) >= MaxWarnings {
			return
		}
		p.Warnings = append(p.Warnings, Warning{Code: code, Message: msg})
	}
	if !in.SMTPBound {
		add(WarnSMTPUnbound, "SMTP listener is not bound")
		add(WarnListenerUnbound, "a required listener is not bound")
	}
	if !in.StoreUp {
		add(WarnStoreDown, "message store is not initialized")
	}
	if !mgmtOK {
		add(WarnMgmtUnbound, "management listener is not bound")
		if in.SMTPBound {
			add(WarnListenerUnbound, "a required listener is not bound")
		}
	}
	return p
}
