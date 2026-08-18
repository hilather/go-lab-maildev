package rest

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
)

type sseTestFrame struct {
	event string
	data  map[string]any
}

func TestEventsStream(t *testing.T) {
	svc, err := app.Boot(context.Background(), app.Options{BootstrapPath: copyDefaults(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	s, err := New(Config{Service: svc, RatePerSec: -1, SSEHeartbeat: 20 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/v1/events/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type=%s", resp.Header.Get("Content-Type"))
	}

	frames := make(chan sseTestFrame, 8)
	heartbeats := make(chan struct{}, 8)
	go func() {
		br := bufio.NewReader(resp.Body)
		var ev string
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case line == ": heartbeat":
				select {
				case heartbeats <- struct{}{}:
				default:
				}
			case strings.HasPrefix(line, "event: "):
				ev = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				var payload map[string]any
				_ = json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &payload)
				frames <- sseTestFrame{event: ev, data: payload}
				ev = ""
			}
		}
	}()

	id := insertMail(t, svc, "sse-sub", "body")
	got := waitFrame(t, frames, app.InboxMailReceived)
	if got.data["id"] != id || got.data["storeGeneration"] == nil {
		t.Fatalf("received=%+v", got)
	}
	if err := svc.DeleteMessage(context.Background(), app.Actor{ID: "t"}, id, app.DeleteIn{}); err != nil {
		t.Fatal(err)
	}
	got = waitFrame(t, frames, app.InboxMailDeleted)
	if got.data["id"] != id {
		t.Fatalf("deleted=%+v", got)
	}
	if _, err := svc.Reset(context.Background(), app.Actor{ID: "t"}, app.ResetIn{Reason: "sse"}); err != nil {
		t.Fatal(err)
	}
	got = waitFrame(t, frames, app.InboxStoreWiped)
	if got.data["storeGeneration"] == nil {
		t.Fatalf("wiped=%+v", got)
	}

	select {
	case <-heartbeats:
	case <-time.After(time.Second):
		t.Fatal("missing : heartbeat comment")
	}
}

func waitFrame(t *testing.T, ch <-chan sseTestFrame, name string) sseTestFrame {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case f := <-ch:
			if f.event == name {
				return f
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", name)
		}
	}
}
