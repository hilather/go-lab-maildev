// Package smtp is the receive-only SMTP data plane.
//
// Line IO lives in smtp/codec. The listener and session state machine live
// in smtp/server. Production files accept inbound TCP only and must not
// import net/smtp, net/http, or internal/control.
package smtp
