package observability

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestLoggerJSONFields(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, LevelInfo).WithSync()
	l.Log(Record{
		Event:           EventSMTPAccepted,
		Component:       "smtp",
		MessageID:       "01TESTMESSAGEID0000000000",
		SMTPCode:        250,
		Result:          "accepted",
		StoreGeneration: 3,
		Remote:          "192.0.2.55",
		Timestamp:       time.Unix(0, 0).UTC(),
	})
	s := buf.String()
	if strings.Contains(s, "192.0.2.55") {
		t.Fatalf("info must not log remote IP: %s", s)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &rec); err != nil {
		t.Fatal(err)
	}
	if rec["event"] != EventSMTPAccepted {
		t.Fatalf("event=%v", rec["event"])
	}
	if rec["smtp_code"] != float64(250) {
		t.Fatalf("smtp_code=%v", rec["smtp_code"])
	}
	if rec["store_generation"] != float64(3) {
		t.Fatalf("store_generation=%v", rec["store_generation"])
	}
	if _, ok := rec["timestamp"]; !ok {
		t.Fatalf("missing timestamp: %s", s)
	}
	for _, bad := range []string{"Authorization", "password", "DATA"} {
		if strings.Contains(s, bad) {
			t.Fatalf("leaked %s: %s", bad, s)
		}
	}
}

func TestLoggerDebugKeepsRemote(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, LevelDebug).WithSync()
	l.Log(Record{Event: EventSMTPSessionEnd, Remote: "192.0.2.9", Timestamp: time.Unix(0, 0).UTC()})
	if !strings.Contains(buf.String(), "192.0.2.9") {
		t.Fatalf("debug mode should keep remote: %s", buf.String())
	}
}

func TestLoggerDefaultWritesSync(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf, LevelInfo)
	l.Log(Record{Event: EventStateReset, Result: "ok"})
	if !strings.Contains(buf.String(), EventStateReset) {
		t.Fatalf("default Log must write: %s", buf.String())
	}
}

func TestLoggerQueueDropDoesNotBlock(t *testing.T) {
	reg := NewRegistry()
	l := NewLogger(nil, LevelInfo).WithQueue(1).WithMetrics(reg)
	if !l.Queue().TrySend(Record{Event: EventHTTPRequest}) {
		t.Fatal("first enqueue")
	}
	done := make(chan struct{})
	go func() {
		l.Log(Record{Event: EventAuthFailure})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Log blocked on full queue")
	}
	if l.Queue().Dropped() == 0 || l.Dropped() == 0 {
		t.Fatal("expected drop")
	}
	if v, ok := reg.Get(MetricTelemetryDropped, map[string]string{"reason": "log"}); !ok || v < 1 {
		t.Fatalf("log overflow not counted: %v ok=%v", v, ok)
	}
}

func TestParseLevel(t *testing.T) {
	if ParseLevel("DEBUG") != LevelDebug || ParseLevel("nope") != LevelInfo {
		t.Fatal("ParseLevel")
	}
}
