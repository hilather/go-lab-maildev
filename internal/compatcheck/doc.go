// Package compatcheck probes a live SMTP+HTTP pair the same way
// mcp-integration-lab smoke talks to maildev: SendMail, GET /email, GET /healthz.
//
// MailDev 2.2.1 and LabMail are both oracles of this client. LabMail is the
// unit under test; documented deltas (ULID ids, sha256 checksums, omitted
// list bodies, relay 403) stay assertions, not failures.
package compatcheck
