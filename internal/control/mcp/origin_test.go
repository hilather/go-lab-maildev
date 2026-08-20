package mcp

import (
	"net/http"
	"testing"
)

func TestOriginMissingAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
	}, "127.0.0.1:1")
	if rec.Code == http.StatusForbidden {
		t.Fatalf("missing Origin rejected: %s", rec.Body.String())
	}
}

func TestOriginLoopbackAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	for _, origin := range []string{"http://127.0.0.1:1080", "http://localhost", "http://[::1]:9"} {
		rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
			"Content-Type":        "application/json",
			"Accept":              "application/json, text/event-stream",
			headerProtocolVersion: ProtocolVersion,
			headerOrigin:          origin,
		}, "127.0.0.1:1")
		if rec.Code == http.StatusForbidden {
			t.Fatalf("loopback Origin %s rejected: %s", origin, rec.Body.String())
		}
	}
}

func TestOriginRemoteDenied(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://evil.example",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusForbidden, "forbidden")
}

func TestOriginAllowlist(t *testing.T) {
	_, svc := newTestServer(t)
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowedOrigins: []string{"https://mgmt.lab.example"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ok := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://mgmt.lab.example",
	}, "127.0.0.1:1")
	if ok.Code == http.StatusForbidden {
		t.Fatalf("allowlisted Origin rejected: %s", ok.Body.String())
	}
	bad := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://other.example",
	}, "127.0.0.1:1")
	requireRPCError(t, bad, http.StatusForbidden, "forbidden")
}

func TestOriginStar(t *testing.T) {
	_, svc := newTestServer(t)
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowedOrigins: []string{"*"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ok := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://evil.example",
	}, "127.0.0.1:1")
	if ok.Code == http.StatusForbidden {
		t.Fatalf("star Origin rejected: %s", ok.Body.String())
	}
	file := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "file://localhost",
	}, "127.0.0.1:1")
	requireRPCError(t, file, http.StatusForbidden, "forbidden")
}

func TestOriginPrivate(t *testing.T) {
	_, svc := newTestServer(t)
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowedOrigins: []string{"private"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	ok := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "http://192.168.1.9:1080",
	}, "127.0.0.1:1")
	if ok.Code == http.StatusForbidden {
		t.Fatalf("private LAN Origin rejected: %s", ok.Body.String())
	}
	bad := doRaw(t, s.Handler(), rpcCall(1, "ping", nil), map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://evil.example",
	}, "127.0.0.1:1")
	requireRPCError(t, bad, http.StatusForbidden, "forbidden")
}

func TestOriginOPTIONSAfterAllowedIs405(t *testing.T) {
	_, svc := newTestServer(t)
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowedOrigins: []string{"*"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	rec := doRawMethod(t, s.Handler(), http.MethodOptions, DefaultPath, "", map[string]string{
		"Accept":              "application/json, text/event-stream",
		headerProtocolVersion: ProtocolVersion,
		headerOrigin:          "https://evil.example",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusMethodNotAllowed, "method_not_allowed")
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("Access-Control-Allow-Origin=%q", v)
	}
}

func TestOriginAllowedHelper(t *testing.T) {
	if originAllowed("https://evil.example", nil) {
		t.Fatal("evil origin allowed")
	}
	if !originAllowed("http://127.0.0.1", nil) {
		t.Fatal("loopback origin denied")
	}
	if originAllowed("file://localhost", nil) {
		t.Fatal("file origin allowed")
	}
	if !originAllowed("https://ok.example", []string{"https://ok.example"}) {
		t.Fatal("allowlist miss")
	}
}
