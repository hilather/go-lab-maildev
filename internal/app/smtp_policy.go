package app

import (
	"fmt"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// rejectUnimplementedSMTP matches smtp/server.New: AUTH and STARTTLS are
// SMTP-001b. Live apply/reset must not install a posture serve would reject.
func rejectUnimplementedSMTP(spec model.SMTPSpec) error {
	switch spec.Auth.Mode {
	case "", model.SMTPAuthNone:
	default:
		return domainerr.ValidationFailed("smtp.auth.mode is not implemented until SMTP-001b",
			domainerr.FieldViolation{
				Path:    "spec.smtp.auth.mode",
				Code:    "invalid_value",
				Message: fmt.Sprintf("smtp.auth.mode %q is not implemented until SMTP-001b", spec.Auth.Mode),
			})
	}
	switch spec.TLS.Mode {
	case "", model.TLSModeOff:
	default:
		return domainerr.ValidationFailed("smtp.tls.mode is not implemented until SMTP-001b",
			domainerr.FieldViolation{
				Path:    "spec.smtp.tls.mode",
				Code:    "invalid_value",
				Message: fmt.Sprintf("smtp.tls.mode %q is not implemented until SMTP-001b", spec.TLS.Mode),
			})
	}
	if spec.TLS.Required {
		return domainerr.ValidationFailed("smtp.tls.required is not implemented until SMTP-001b",
			domainerr.FieldViolation{
				Path:    "spec.smtp.tls.required",
				Code:    "invalid_value",
				Message: "smtp.tls.required is not implemented until SMTP-001b",
			})
	}
	return nil
}
