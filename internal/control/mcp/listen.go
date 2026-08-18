package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

const (
	resourceMessages = "labmail://messages"
	listenHeartbeat  = 15 * time.Second
)

type listenNotifications struct {
	ToolsListChanged      bool     `json:"toolsListChanged"`
	PromptsListChanged    bool     `json:"promptsListChanged"`
	ResourcesListChanged  bool     `json:"resourcesListChanged"`
	ResourceSubscriptions []string `json:"resourceSubscriptions"`
}

// handleListen implements subscriptions/listen. Notifications are URI-only;
// clients pull bodies with mail_messages_list. The pin stays 2026-07-28 even
// when allowLegacyClients is true (D17).
func (s *Server) handleListen(w http.ResponseWriter, r *http.Request) {
	headerVer := strings.TrimSpace(r.Header.Get(headerProtocolVersion))

	body, err := io.ReadAll(io.LimitReader(r.Body, s.maxBody+1))
	if err != nil {
		writeRPC(w, http.StatusBadRequest, domainerr.ValidationFailed("parse error"))
		return
	}
	if int64(len(body)) > s.maxBody {
		writeRPC(w, http.StatusBadRequest, domainerr.ValidationFailed("request body too large"))
		return
	}
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if len(bytesTrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeRPC(w, http.StatusBadRequest, domainerr.ValidationFailed("parse error"))
			return
		}
	}
	if req.Method != "" && req.Method != methodListen {
		writeRPC(w, http.StatusBadRequest, domainerr.ValidationFailed("Mcp-Method does not match body method"))
		return
	}

	var params struct {
		Meta          map[string]any      `json:"_meta"`
		Notifications listenNotifications `json:"notifications"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			writeRPC(w, http.StatusBadRequest, domainerr.ValidationFailed("invalid listen params"))
			return
		}
	}
	metaVer, _ := params.Meta["io.modelcontextprotocol/protocolVersion"].(string)
	if headerVer != ProtocolVersion && metaVer != ProtocolVersion {
		writeRPC(w, http.StatusBadRequest, domainerr.ValidationFailed("subscriptions/listen requires protocol "+ProtocolVersion))
		return
	}

	accepted := listenNotifications{}
	var wantMessages bool
	for _, uri := range params.Notifications.ResourceSubscriptions {
		if uri == resourceMessages {
			accepted.ResourceSubscriptions = append(accepted.ResourceSubscriptions, uri)
			wantMessages = true
		}
	}

	actor := actorFrom(r.Context())
	if err := s.authorizeResource(actor, resourceMessages); err != nil {
		writeRPC(w, http.StatusForbidden, err)
		return
	}

	events := (<-chan struct{})(nil)
	cancel := func() {}
	if wantMessages {
		ch, stop := s.svc.Subscribe(r.Context(), actor, 16)
		cancel = stop
		notify := make(chan struct{}, 1)
		go func() {
			defer close(notify)
			for range ch {
				select {
				case notify <- struct{}{}:
				default:
				}
			}
		}()
		events = notify
	}
	defer cancel()

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}

	subID := jsonRaw(req.ID)
	writeSSEJSON(w, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/subscriptions/acknowledged",
		"params": map[string]any{
			"_meta":         map[string]any{"io.modelcontextprotocol/subscriptionId": subID},
			"notifications": ackFilter(accepted),
		},
	})
	flush()
	_, _ = io.WriteString(w, ": keepalive\n\n")
	flush()

	tick := time.NewTicker(listenHeartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			writeSSEJSON(w, listenComplete(req.ID, subID))
			flush()
			return
		case <-tick.C:
			_, _ = io.WriteString(w, ": keepalive\n\n")
			flush()
		case _, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if wantMessages {
				writeSSEJSON(w, map[string]any{
					"jsonrpc": "2.0",
					"method":  "notifications/resources/updated",
					"params": map[string]any{
						"_meta": map[string]any{"io.modelcontextprotocol/subscriptionId": subID},
						"uri":   resourceMessages,
					},
				})
				flush()
			}
		}
	}
}

func ackFilter(n listenNotifications) map[string]any {
	out := map[string]any{}
	if len(n.ResourceSubscriptions) > 0 {
		out["resourceSubscriptions"] = n.ResourceSubscriptions
	}
	return out
}

func listenComplete(id json.RawMessage, subID any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      jsonRaw(id),
		"result": map[string]any{
			"resultType": "complete",
			"_meta":      map[string]any{"io.modelcontextprotocol/subscriptionId": subID},
		},
	}
}

func jsonRaw(id json.RawMessage) any {
	var v any
	if err := json.Unmarshal(id, &v); err != nil {
		return nil
	}
	return v
}

func writeSSEJSON(w http.ResponseWriter, v any) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}
	_, _ = io.WriteString(w, "data: ")
	_, _ = w.Write(raw)
	_, _ = io.WriteString(w, "\n\n")
}

func bytesTrimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}
