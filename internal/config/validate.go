package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// Validate checks a (preferably normalized) state. It does not mutate st.
func Validate(st *model.State) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	var vs []domainerr.FieldViolation
	validateDocument(st, &vs)
	validateListeners(&st.Spec.Listeners, &vs)
	validateSMTP(&st.Spec.SMTP, &vs)
	validateStore(&st.Spec.Store, &vs)
	validateManagement(&st.Spec.Management, &vs)
	validateObservability(&st.Spec.Observability, &vs)
	if len(vs) > 0 {
		return domainerr.ValidationFailed("Candidate state is invalid.", vs...)
	}
	return nil
}

func validateDocument(st *model.State, vs *[]domainerr.FieldViolation) {
	if st.APIVersion != model.APIVersionV1Alpha1 {
		code := violationUnsupportedVersion
		msg := fmt.Sprintf("apiVersion must be %q", model.APIVersionV1Alpha1)
		if strings.TrimSpace(st.APIVersion) == "" {
			code = violationRequired
			msg = "apiVersion is required"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: "apiVersion", Code: code, Message: msg})
	}
	if st.Kind != model.KindLabMail {
		code := violationInvalidValue
		msg := fmt.Sprintf("kind must be %q", model.KindLabMail)
		if strings.TrimSpace(st.Kind) == "" {
			code = violationRequired
			msg = "kind is required"
		}
		*vs = append(*vs, domainerr.FieldViolation{Path: "kind", Code: code, Message: msg})
	}
	if strings.TrimSpace(st.Metadata.Name) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "metadata.name",
			Code:    violationRequired,
			Message: "metadata.name is required",
		})
	}
}

func validateListeners(l *model.ListenersSpec, vs *[]domainerr.FieldViolation) {
	if l.SMTP.Address != "" {
		if _, _, err := net.SplitHostPort(l.SMTP.Address); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.listeners.smtp.address",
				Code:    violationInvalidValue,
				Message: "SMTP listen address must be host:port",
			})
		}
	}
	if l.Management.Address != "" {
		if _, _, err := net.SplitHostPort(l.Management.Address); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.listeners.management.address",
				Code:    violationInvalidValue,
				Message: "management listen address must be host:port",
			})
		}
	}
	if l.Management.RESTPath != "" && !strings.HasPrefix(l.Management.RESTPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.restPath",
			Code:    violationInvalidValue,
			Message: "restPath must start with /",
		})
	}
	if l.Management.MCPPath != "" && !strings.HasPrefix(l.Management.MCPPath, "/") {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.listeners.management.mcpPath",
			Code:    violationInvalidValue,
			Message: "mcpPath must start with /",
		})
	}
	validateFilePair("spec.listeners.management.tls", l.Management.TLS.Enabled, l.Management.TLS.CertFile, l.Management.TLS.KeyFile, vs)
}

func validateSMTP(s *model.SMTPSpec, vs *[]domainerr.FieldViolation) {
	if strings.TrimSpace(s.Hostname) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.smtp.hostname",
			Code:    violationRequired,
			Message: "smtp.hostname is required",
		})
	}
	if s.MaxMessageBytes <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.smtp.maxMessageBytes",
			Code:    violationInvalidValue,
			Message: "maxMessageBytes must be > 0 (unbounded is not a lab mode)",
		})
	}
	if s.MaxRecipients <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.smtp.maxRecipients",
			Code:    violationInvalidValue,
			Message: "maxRecipients must be > 0",
		})
	}
	for i, ext := range s.HideExtensions {
		if strings.TrimSpace(ext) == "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    indexPath("spec.smtp.hideExtensions", i),
				Code:    violationInvalidValue,
				Message: "hideExtensions entries must be non-empty",
			})
		}
	}
	validateSMTPAuth(&s.Auth, vs)
	validateSMTPTLS(&s.TLS, vs)
	validateAdmission(&s.Admission, vs)
}

func validateSMTPAuth(a *model.SMTPAuthSpec, vs *[]domainerr.FieldViolation) {
	switch a.Mode {
	case "", model.SMTPAuthNone:
		if a.Username != "" || a.PasswordFile != "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.smtp.auth",
				Code:    violationInvalidValue,
				Message: "smtp.auth username/passwordFile are illegal when mode is none",
			})
		}
	case model.SMTPAuthPlainLogin:
		if strings.TrimSpace(a.Username) == "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.smtp.auth.username",
				Code:    violationRequired,
				Message: "plain_login requires username",
			})
		}
		if strings.TrimSpace(a.PasswordFile) == "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.smtp.auth.passwordFile",
				Code:    violationRequired,
				Message: "plain_login requires passwordFile",
			})
		} else {
			requireExistingFile("spec.smtp.auth.passwordFile", a.PasswordFile, vs)
		}
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.smtp.auth.mode",
			Code:    violationInvalidValue,
			Message: "smtp.auth.mode must be none or plain_login",
		})
	}
}

func validateSMTPTLS(t *model.SMTPTLSSpec, vs *[]domainerr.FieldViolation) {
	switch t.Mode {
	case "", model.TLSModeOff:
		if t.Required {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.smtp.tls.required",
				Code:    violationInvalidValue,
				Message: "required is legal only when mode is starttls",
			})
		}
		if t.CertFile != "" || t.KeyFile != "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.smtp.tls",
				Code:    violationInvalidValue,
				Message: "smtp.tls certFile/keyFile are illegal when mode is off",
			})
		}
	case model.TLSModeStartTLS:
		if strings.TrimSpace(t.CertFile) == "" || strings.TrimSpace(t.KeyFile) == "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.smtp.tls",
				Code:    violationRequired,
				Message: "mode starttls requires certFile and keyFile",
			})
		} else {
			requireExistingFile("spec.smtp.tls.certFile", t.CertFile, vs)
			requireExistingFile("spec.smtp.tls.keyFile", t.KeyFile, vs)
		}
	case model.TLSModeImplicit:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.smtp.tls.mode",
			Code:    violationInvalidValue,
			Message: "smtp.tls.mode: implicit is not supported until 1.1; use starttls or a future listeners.smtpImplicit bind",
		})
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.smtp.tls.mode",
			Code:    violationInvalidValue,
			Message: "smtp.tls.mode must be off, starttls, or implicit",
		})
	}
}

func validateAdmission(a *model.AdmissionSpec, vs *[]domainerr.FieldViolation) {
	if a.MaxSessions <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.maxSessions", Code: violationInvalidValue, Message: "maxSessions must be > 0"})
	}
	if a.MaxSessionsPerIP <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.maxSessionsPerIP", Code: violationInvalidValue, Message: "maxSessionsPerIP must be > 0"})
	}
	if a.MaxInFlightData <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.maxInFlightData", Code: violationInvalidValue, Message: "maxInFlightData must be > 0"})
	}
	if a.MaxInFlightDataBytes <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.maxInFlightDataBytes", Code: violationInvalidValue, Message: "maxInFlightDataBytes must be > 0"})
	}
	if a.SessionTimeout <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.sessionTimeout", Code: violationInvalidValue, Message: "sessionTimeout must be > 0"})
	}
	if a.CommandIdle <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.commandIdle", Code: violationInvalidValue, Message: "commandIdle must be > 0"})
	}
	if a.DataIdle <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.smtp.admission.dataIdle", Code: violationInvalidValue, Message: "dataIdle must be > 0"})
	}
}

func validateStore(s *model.StoreSpec, vs *[]domainerr.FieldViolation) {
	if s.MaxMessages <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.store.maxMessages", Code: violationInvalidValue, Message: "maxMessages must be > 0"})
	}
	if s.MaxBytes <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.store.maxBytes", Code: violationInvalidValue, Message: "maxBytes must be > 0"})
	}
	switch s.FullPolicy {
	case "", model.FullPolicyReject, model.FullPolicyEvictOldest:
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.store.fullPolicy",
			Code:    violationInvalidValue,
			Message: "fullPolicy must be reject or evict_oldest",
		})
	}
	if s.MaxWait < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.store.maxWait", Code: violationInvalidValue, Message: "maxWait must be >= 0"})
	}
	if s.SpillThreshold < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.store.spillThreshold", Code: violationInvalidValue, Message: "spillThreshold must be >= 0"})
	}
}

func validateManagement(m *model.ManagementSpec, vs *[]domainerr.FieldViolation) {
	switch m.Auth.Mode {
	case "", model.MgmtAuthBearer, model.MgmtAuthBearerAndBasic, model.MgmtAuthDevLoopbackUnauth:
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.management.auth.mode",
			Code:    violationInvalidValue,
			Message: "management.auth.mode must be bearer, bearer_and_basic, or dev-loopback-unauth",
		})
	}
	ids := map[string]string{}
	for i, tok := range m.Auth.Tokens {
		path := indexPath("spec.management.auth.tokens", i)
		id := strings.TrimSpace(tok.ID)
		if id == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".id", Code: violationEmptyID, Message: "token id is required"})
		} else if prev, ok := ids[id]; ok {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".id", Code: violationDuplicateID, Message: "duplicate token id (first at " + prev + ")"})
		} else {
			ids[id] = path + ".id"
		}
		if strings.TrimSpace(tok.SecretFile) == "" {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".secretFile", Code: violationRequired, Message: "token secretFile is required"})
		} else {
			checkTokenSecretLength(path+".secretFile", tok.SecretFile, vs)
		}
		if tok.Role != "" && !model.KnownRole(tok.Role) {
			*vs = append(*vs, domainerr.FieldViolation{Path: path + ".role", Code: violationInvalidValue, Message: "role must be viewer, operator, or administrator"})
		}
		for si, sc := range tok.Scopes {
			if !model.KnownScope(sc) {
				*vs = append(*vs, domainerr.FieldViolation{
					Path:    indexPath(path+".scopes", si),
					Code:    violationInvalidValue,
					Message: fmt.Sprintf("unknown scope %q", sc),
				})
			}
		}
	}
	basic := m.Auth.Basic
	if basic.Username != "" || basic.PasswordFile != "" || basic.TokenRef != "" {
		if strings.TrimSpace(basic.Username) == "" || strings.TrimSpace(basic.PasswordFile) == "" || strings.TrimSpace(basic.TokenRef) == "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.management.auth.basic",
				Code:    violationRequired,
				Message: "basic requires username, passwordFile, and tokenRef",
			})
		} else if _, ok := ids[basic.TokenRef]; !ok {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.management.auth.basic.tokenRef",
				Code:    violationUnresolved,
				Message: "basic.tokenRef " + basic.TokenRef + " does not match a token id",
			})
		}
	}
	if m.BodyLimit <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.bodyLimit", Code: violationInvalidValue, Message: "bodyLimit must be > 0"})
	}
	if m.RequestsPerSecond <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.requestsPerSecond", Code: violationInvalidValue, Message: "requestsPerSecond must be > 0"})
	}
	if m.Burst <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.burst", Code: violationInvalidValue, Message: "burst must be > 0"})
	}
	if m.MaxConcurrent <= 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.management.maxConcurrent", Code: violationInvalidValue, Message: "maxConcurrent must be > 0"})
	}
}

func validateObservability(o *model.ObservabilitySpec, vs *[]domainerr.FieldViolation) {
	switch o.LogLevel {
	case "", model.LogLevelDebug, model.LogLevelInfo, model.LogLevelWarn, model.LogLevelError:
	default:
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    "spec.observability.logLevel",
			Code:    violationInvalidValue,
			Message: "logLevel must be debug, info, warn, or error",
		})
	}
	if o.Metrics.Listen != "" {
		if _, _, err := net.SplitHostPort(o.Metrics.Listen); err != nil {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    "spec.observability.metrics.listen",
				Code:    violationInvalidValue,
				Message: "metrics.listen must be host:port or empty",
			})
		}
	}
	if o.Audit.Ring < 0 {
		*vs = append(*vs, domainerr.FieldViolation{Path: "spec.observability.audit.ring", Code: violationInvalidValue, Message: "audit.ring must be >= 0"})
	}
}

func validateFilePair(path string, enabled bool, cert, key string, vs *[]domainerr.FieldViolation) {
	if !enabled {
		if cert != "" || key != "" {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path,
				Code:    violationInvalidValue,
				Message: "certFile/keyFile are illegal when TLS is disabled",
			})
		}
		return
	}
	if strings.TrimSpace(cert) == "" || strings.TrimSpace(key) == "" {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    path,
			Code:    violationRequired,
			Message: "enabled TLS requires certFile and keyFile",
		})
		return
	}
	requireExistingFile(path+".certFile", cert, vs)
	requireExistingFile(path+".keyFile", key, vs)
}

func requireExistingFile(path, file string, vs *[]domainerr.FieldViolation) {
	if _, err := os.Stat(file); err != nil {
		*vs = append(*vs, domainerr.FieldViolation{
			Path:    path,
			Code:    violationUnresolved,
			Message: "file does not resolve at load",
		})
	}
}

// checkTokenSecretLength fails if the file exists and the first secret line
// is shorter than 32 bytes so validate matches serve/FromSpec.
func checkTokenSecretLength(path, file string, vs *[]domainerr.FieldViolation) {
	b, err := os.ReadFile(file)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) < 32 {
			*vs = append(*vs, domainerr.FieldViolation{
				Path:    path,
				Code:    violationInvalidValue,
				Message: "token secret must be at least 32 bytes",
			})
		}
		return
	}
	*vs = append(*vs, domainerr.FieldViolation{
		Path:    path,
		Code:    violationInvalidValue,
		Message: "token secret must be at least 32 bytes",
	})
}
