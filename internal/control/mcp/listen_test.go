package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListenAcknowledgesMessagesURI(t *testing.T) {
	s, svc := newTestServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := `{"jsonrpc":"2.0","id":7,"method":"subscriptions/listen","params":{"notifications":{"resourceSubscriptions":["labmail://messages"]}}}`
	req := httptest.NewRequest(http.MethodPost, DefaultPath, strings.NewReader(body)).WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(headerMethod, methodListen)
	req.Header.Set(headerProtocolVersion, ProtocolVersion)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Handler().ServeHTTP(rec, req)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.Body.String(), "notifications/subscriptions/acknowledged") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(rec.Body.String(), "labmail://messages") {
		t.Fatalf("ack missing uri: %s", rec.Body.String())
	}

	insertMail(t, svc, "listen", "body")
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.Body.String(), "notifications/resources/updated") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.Contains(rec.Body.String(), `"uri":"labmail://messages"`) {
		t.Fatalf("missing URI-only notify: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"subject":"listen"`) {
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
