// Package codec is SMTP line IO and reply formatting.
//
// It does not own sessions, TLS, or the store. Command lines are capped at
// 512 octets including CRLF (RFC 5321 §4.5.3.1.4). DATA lines are capped at
// 8192 octets.
package codec
