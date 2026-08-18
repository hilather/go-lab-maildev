package mcp

import (
	"net/http"
	"testing"
)

func TestBasicRejected(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Basic YWRtaW46c2VjcmV0",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("missing WWW-Authenticate")
	}
}

func TestBearerStubAccepted(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerAuthorization:   "Bearer stub-token",
	}, "127.0.0.1:1")
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("bearer rejected: %s", rec.Body.String())
	}
}

func TestMCPAuthVerifierBearerOnly(t *testing.T) {
	s, _ := newTestServer(t)
	v, token := testVerifier(t)
	s.cfg.Auth = v
	hdr := map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}
	hdr[headerAuthorization] = "Basic YWRtaW46c2VjcmV0"
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), hdr, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")

	hdr[headerAuthorization] = "Bearer " + token
	rec = doRaw(t, s.Handler(), rpcCall(1, "ping", nil), hdr, "127.0.0.1:1")
	if rec.Code == http.StatusUnauthorized {
		t.Fatalf("valid bearer rejected: %s", rec.Body.String())
	}

	delete(hdr, headerAuthorization)
	rec = doRaw(t, s.Handler(), rpcCall(1, "ping", nil), hdr, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusUnauthorized, "unauthenticated")
}
