package mcp

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// safeRecorder lets a test poll an SSE body while the handler writes it.
// httptest.ResponseRecorder is not safe for that concurrent use.
type safeRecorder struct {
	mu          sync.Mutex
	header      http.Header
	code        int
	wroteHeader bool
	body        bytes.Buffer
}

func newSafeRecorder() *safeRecorder {
	return &safeRecorder{header: make(http.Header)}
}

func (r *safeRecorder) Header() http.Header { return r.header }

func (r *safeRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.wroteHeader {
		return
	}
	r.code = status
	r.wroteHeader = true
}

func (r *safeRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.wroteHeader {
		r.code = http.StatusOK
		r.wroteHeader = true
	}
	return r.body.Write(p)
}

func (r *safeRecorder) Flush() {}

func (r *safeRecorder) snapshot() (code int, body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.code, r.body.String()
}

func listenRPC(id int, proto string) string {
	return rpcCall(id, methodListen, map[string]any{
		"_meta": map[string]any{
			"io.modelcontextprotocol/protocolVersion": proto,
			"io.modelcontextprotocol/clientInfo": map[string]any{
				"name": "labmail-test", "version": "dev",
			},
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		},
		"notifications": map[string]any{
			"resourceSubscriptions": []string{resourceMessages},
		},
	})
}

func TestListenAcknowledgesMessagesURI(t *testing.T) {
	s, svc := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := listenRPC(7, ProtocolVersion)
	req := httptest.NewRequest(http.MethodPost, DefaultPath, strings.NewReader(body)).WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(headerMethod, methodListen)
	req.Header.Set(headerProtocolVersion, ProtocolVersion)
	rec := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(rec, req)
	}()

	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		_, got = rec.snapshot()
		if strings.Contains(got, "notifications/subscriptions/acknowledged") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, got = rec.snapshot()
	if !strings.Contains(got, "labmail://messages") {
		t.Fatalf("ack missing uri: %s", got)
	}

	insertMail(t, svc, "listen", "body")
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, got = rec.snapshot()
		if strings.Contains(got, "notifications/resources/updated") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, got = rec.snapshot()
	if !strings.Contains(got, `"uri":"labmail://messages"`) {
		t.Fatalf("missing URI-only notify: %s", got)
	}
	if strings.Contains(got, `"subject":"listen"`) {
		t.Fatal("listen must not include message bodies")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("listen did not return")
	}
}

func TestListenRequiresPinnedProtocol(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRaw(t, s.Handler(), `{"jsonrpc":"2.0","id":1,"method":"subscriptions/listen","params":{}}`, map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "text/event-stream",
		headerMethod:          methodListen,
		headerProtocolVersion: "2025-11-25",
	}, "127.0.0.1:1")
	requireRPCError(t, rec, http.StatusBadRequest, "validation_failed")
}

func TestListenPinWithLegacyClients(t *testing.T) {
	_, svc := newTestServer(t)
	s, err := New(Config{Service: svc, RatePerSec: -1, AllowLegacyClients: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"legacy-gateway","version":"0.0.1"}}}`
	initRec := doRaw(t, s.Handler(), initBody, map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json, text/event-stream",
	}, "127.0.0.1:1")
	if initRec.Code != http.StatusOK && initRec.Code != http.StatusAccepted {
		t.Fatalf("legacy initialize status=%d body=%s", initRec.Code, initRec.Body.String())
	}
	initRPC := decodeRPC(t, initRec)
	if _, ok := initRPC["error"]; ok {
		t.Fatalf("legacy initialize error: %s", initRec.Body.String())
	}
	result, _ := initRPC["result"].(map[string]any)
	if result == nil || result["protocolVersion"] == "" {
		t.Fatalf("legacy initialize missing protocolVersion: %s", initRec.Body.String())
	}

	badHeader := doRaw(t, s.Handler(), `{"jsonrpc":"2.0","id":2,"method":"subscriptions/listen","params":{"notifications":{"resourceSubscriptions":["labmail://messages"]}}}`, map[string]string{
		"Content-Type":        "application/json",
		"Accept":              "text/event-stream",
		headerMethod:          methodListen,
		headerProtocolVersion: "2025-11-25",
	}, "127.0.0.1:1")
	requireRPCError(t, badHeader, http.StatusBadRequest, "validation_failed")

	missing := doRaw(t, s.Handler(), `{"jsonrpc":"2.0","id":3,"method":"subscriptions/listen","params":{"notifications":{"resourceSubscriptions":["labmail://messages"]}}}`, map[string]string{
		"Content-Type": "application/json",
		"Accept":       "text/event-stream",
		headerMethod:   methodListen,
	}, "127.0.0.1:1")
	requireRPCError(t, missing, http.StatusBadRequest, "validation_failed")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	metaBody := listenRPC(4, ProtocolVersion)
	req := httptest.NewRequest(http.MethodPost, DefaultPath, strings.NewReader(metaBody)).WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(headerMethod, methodListen)
	rec := newSafeRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(rec, req)
	}()
	deadline := time.Now().Add(2 * time.Second)
	var code int
	var got string
	for time.Now().Before(deadline) {
		code, got = rec.snapshot()
		if code == http.StatusBadRequest {
			t.Fatalf("listen with only _meta pin rejected: %s", got)
		}
		if strings.Contains(got, "notifications/subscriptions/acknowledged") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	code, got = rec.snapshot()
	if code == http.StatusBadRequest {
		t.Fatalf("listen with only _meta pin rejected: %s", got)
	}
	if !strings.Contains(got, "notifications/subscriptions/acknowledged") {
		t.Fatalf("listen with only _meta pin did not ack: status=%d body=%s", code, got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("meta-pin listen did not return")
	}
}
