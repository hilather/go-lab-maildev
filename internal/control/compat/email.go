package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/mimeparse"
	"github.com/hilather/go-lab-maildev/internal/model"
	"github.com/hilather/go-lab-maildev/internal/preview"
)

type emailJSON struct {
	ID          string            `json:"id"`
	Time        string            `json:"time"`
	From        []addrJSON        `json:"from"`
	To          []addrJSON        `json:"to"`
	Cc          []addrJSON        `json:"cc"`
	Bcc         []addrJSON        `json:"bcc"`
	Subject     string            `json:"subject"`
	Text        string            `json:"text"`
	HTML        string            `json:"html"`
	Headers     map[string]string `json:"headers"`
	Read        bool              `json:"read"`
	MessageID   string            `json:"messageId"`
	Priority    string            `json:"priority"`
	Attachments []attJSON         `json:"attachments"`
	Envelope    envelopeJSON      `json:"envelope"`
}

type addrJSON struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type attJSON struct {
	FileName           string `json:"fileName"`
	ContentType        string `json:"contentType"`
	ContentDisposition string `json:"contentDisposition"`
	ContentID          string `json:"contentId"`
	Checksum           string `json:"checksum"`
}

type envelopeJSON struct {
	From          string   `json:"from"`
	To            []string `json:"to"`
	Host          string   `json:"host"`
	RemoteAddress string   `json:"remoteAddress"`
}

type configJSON struct {
	SMTP        listenerJSON `json:"smtp"`
	Web         listenerJSON `json:"web"`
	ReceiveOnly bool         `json:"receiveOnly"`
	Hostname    string       `json:"hostname"`
}

type listenerJSON struct {
	Address string `json:"address"`
}

type healthJSON struct {
	Status string `json:"status"`
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	qs := r.URL.Query()
	filter := model.MessageFilter{Subject: qs.Get("subject")}
	msgs, err := h.listAll(r.Context(), actor, filter)
	if err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	textPrefix := qs.Get("text") == "1"
	items := make([]emailJSON, 0, len(msgs))
	for _, m := range msgs {
		item := fromEmail(m, true, textPrefix)
		if matchQuery(item, qs) {
			items = append(items, item)
		}
	}
	skip := parseSkip(qs.Get("skip"))
	if skip > len(items) {
		skip = len(items)
	}
	h.writeJSON(w, http.StatusOK, items[skip:])
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, id string) {
	// maildev GET /email/:id marks the message read. Native /v1 defaults false.
	msg, err := h.svc.GetMessage(r.Context(), actor, id, true)
	if err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	h.writeJSON(w, http.StatusOK, fromEmail(msg, false, false))
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, id string) {
	if err := h.svc.DeleteMessage(r.Context(), actor, id, app.DeleteIn{}); err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	h.writeJSON(w, http.StatusOK, true)
}

func (h *Handler) handleClear(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	if _, err := h.svc.ClearMessages(r.Context(), actor, app.DeleteIn{}); err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	h.writeJSON(w, http.StatusOK, true)
}

func (h *Handler) handleHTML(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, id string) {
	// maildev getEmailHtml goes through getEmailById, which marks read.
	msg, err := h.svc.GetMessage(r.Context(), actor, id, true)
	if err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	// Same document posture as /v1/.../preview: CSP plus cid: → data: rewrite.
	body := preview.RewriteCID(msg.HTML, msg.Attachments)
	w.Header().Set("Content-Security-Policy", preview.CSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	h.writeBytes(w, http.StatusOK, "text/html; charset=utf-8", []byte(body))
}

func (h *Handler) handleAttachment(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor, id, filename string) {
	msg, err := h.svc.GetMessage(r.Context(), actor, id, false)
	if err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	want := mimeparse.SanitizeFilename(filename)
	var att *model.Attachment
	for i := range msg.Attachments {
		if msg.Attachments[i].Filename == want {
			att = &msg.Attachments[i]
			break
		}
	}
	if att == nil {
		h.writeProblem(w, r, instance, domainerr.NotFound("attachment not found"))
		return
	}
	ct := att.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	name := mimeparse.SanitizeFilename(att.Filename)
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Data)
}

func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request, instance string) {
	if !h.isReady(r.Context()) {
		h.writeJSON(w, http.StatusServiceUnavailable, healthJSON{Status: "not ready"})
		return
	}
	h.writeJSON(w, http.StatusOK, healthJSON{Status: "ok"})
	_ = instance
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request, instance string, actor app.Actor) {
	view, err := h.svc.GetState(r.Context(), actor)
	if err != nil {
		h.writeProblem(w, r, instance, asDomain(err))
		return
	}
	out := configJSON{ReceiveOnly: true}
	if view != nil && view.Canonical != nil {
		out.SMTP.Address = view.Canonical.Spec.Listeners.SMTP.Address
		out.Web.Address = view.Canonical.Spec.Listeners.Management.Address
		out.Hostname = view.Canonical.Spec.SMTP.Hostname
	}
	h.writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listAll(ctx context.Context, actor app.Actor, filter model.MessageFilter) ([]*model.Message, error) {
	maxPages := listPageBudget(0)
	if view, err := h.svc.GetState(ctx, actor); err == nil && view != nil && view.Canonical != nil {
		maxPages = listPageBudget(view.Canonical.Spec.Store.MaxMessages)
	}
	var out []*model.Message
	cursor := ""
	for page := 0; page < maxPages; page++ {
		res, err := h.svc.ListMessages(ctx, actor, model.ListQuery{
			Filter: filter,
			Cursor: cursor,
			Limit:  listPage,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, res.Items...)
		if res.NextCursor == "" || res.NextCursor == cursor {
			return out, nil
		}
		cursor = res.NextCursor
	}
	return out, nil
}

func listPageBudget(maxMessages int) int {
	if maxMessages <= 0 {
		maxMessages = 1000
	}
	return (maxMessages+listPage-1)/listPage + 1
}

func fromEmail(m *model.Message, list bool, textPrefix bool) emailJSON {
	if m == nil {
		return emailJSON{
			From: []addrJSON{}, To: []addrJSON{}, Cc: []addrJSON{}, Bcc: []addrJSON{},
			Headers: map[string]string{}, Attachments: []attJSON{}, Envelope: envelopeJSON{To: []string{}},
		}
	}
	out := emailJSON{
		ID:          m.ID,
		Time:        formatTime(m.ReceivedAt),
		From:        fromAddrs(m.From),
		To:          fromAddrs(m.To),
		Cc:          fromAddrs(m.Cc),
		Bcc:         fromAddrs(m.Bcc),
		Subject:     m.Subject,
		Headers:     headerMap(m.Headers),
		Read:        m.Read,
		MessageID:   strings.Trim(m.MessageID, "<>"),
		Priority:    m.Priority,
		Attachments: fromAtts(m.Attachments),
		Envelope: envelopeJSON{
			From:          m.Envelope.From,
			To:            nonNilStrings(m.Envelope.To),
			Host:          m.Envelope.HELO,
			RemoteAddress: m.Envelope.RemoteAddr,
		},
	}
	if out.Priority == "" {
		out.Priority = "normal"
	}
	if list {
		// Omit bodies so a full inbox cannot serialize hundreds of MiB.
		out.Text = ""
		out.HTML = ""
		if textPrefix {
			out.Text = prefixBytes(m.Text, textPrefixBytes)
		}
		return out
	}
	out.Text = m.Text
	out.HTML = m.HTML
	return out
}

func fromAddrs(in []model.Address) []addrJSON {
	out := make([]addrJSON, 0, len(in))
	for _, a := range in {
		out = append(out, addrJSON{Address: a.Address, Name: a.Name})
	}
	return out
}

func fromAtts(in []model.Attachment) []attJSON {
	out := make([]attJSON, 0, len(in))
	for _, a := range in {
		out = append(out, attJSON{
			FileName:           a.Filename,
			ContentType:        a.ContentType,
			ContentDisposition: a.Disposition,
			ContentID:          a.ContentID,
			Checksum:           a.Checksum,
		})
	}
	return out
}

func headerMap(in []model.Header) map[string]string {
	out := make(map[string]string, len(in))
	for _, h := range in {
		k := strings.ToLower(h.Name)
		if prev, ok := out[k]; ok {
			out[k] = prev + "\n" + h.Value
			continue
		}
		out[k] = h.Value
	}
	return out
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func prefixBytes(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	// Cut at a rune start so ?text=1 does not emit invalid UTF-8.
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func parseSkip(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func matchQuery(item emailJSON, qs map[string][]string) bool {
	raw, err := json.Marshal(item)
	if err != nil {
		return false
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return false
	}
	for key, vals := range qs {
		switch key {
		case "skip", "text":
			continue
		}
		for _, want := range vals {
			if !valueMatches(lookupPath(tree, key), want) {
				return false
			}
		}
	}
	return true
}

func lookupPath(v any, path string) any {
	if path == "" {
		return v
	}
	dot := strings.IndexByte(path, '.')
	head, rest := path, ""
	if dot >= 0 {
		head, rest = path[:dot], path[dot+1:]
	}
	switch cur := v.(type) {
	case map[string]any:
		next, ok := cur[head]
		if !ok {
			return nil
		}
		if rest == "" {
			return next
		}
		return lookupPath(next, rest)
	case []any:
		out := make([]any, 0, len(cur))
		for _, el := range cur {
			out = append(out, lookupPath(el, path))
		}
		return out
	default:
		return nil
	}
}

func valueMatches(got any, want string) bool {
	switch g := got.(type) {
	case nil:
		return false
	case string:
		return g == want
	case bool:
		return strconv.FormatBool(g) == want
	case json.Number:
		return g.String() == want
	case float64:
		return strconv.FormatFloat(g, 'f', -1, 64) == want
	case []any:
		for _, el := range g {
			if valueMatches(el, want) {
				return true
			}
		}
		return false
	default:
		return fmt.Sprint(g) == want
	}
}
