package app

import (
	"fmt"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// rejectUnimplementedSMTP matches smtp/server.New: implicit SMTPS is 1.1.
func rejectUnimplementedSMTP(spec model.SMTPSpec) error {
	switch spec.Auth.Mode {
	case "", model.SMTPAuthNone, model.SMTPAuthPlainLogin:
	default:
		return domainerr.ValidationFailed("unsupported smtp.auth.mode",
			domainerr.FieldViolation{
				Path:    "spec.smtp.auth.mode",
				Code:    "invalid_value",
				Message: fmt.Sprintf("smtp.auth.mode %q is not supported", spec.Auth.Mode),
			})
	}
	switch spec.TLS.Mode {
	case "", model.TLSModeOff, model.TLSModeStartTLS:
	default:
		return domainerr.ValidationFailed("smtp.tls.mode implicit is not implemented in 1.0",
			domainerr.FieldViolation{
				Path:    "spec.smtp.tls.mode",
				Code:    "invalid_value",
				Message: fmt.Sprintf("smtp.tls.mode %q is not implemented in 1.0", spec.TLS.Mode),
			})
	}
	return nil
}
