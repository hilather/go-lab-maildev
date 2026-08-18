package rest

import (
	"context"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	q, err := listQueryFromRequest(r)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	rawCursor := q.Cursor
	var cursorGen uint64
	if rawCursor != "" {
		id, gen, err := s.decodeCursor(rawCursor)
		if err != nil {
			s.writeProblem(w, r, instance, err)
			return
		}
		q.Cursor = id
		cursorGen = gen
	}
	res, err := s.svc.ListMessages(ctx, actor, q)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	if rawCursor != "" && cursorGen != res.Generation {
		s.writeProblem(w, r, instance, domainerr.CursorStale("list cursor is stale; restart the list"))
		return
	}
	items := make([]messageJSON, 0, len(res.Items))
	for _, m := range res.Items {
		items = append(items, fromMessage(m, true))
	}
	var next *string
	if res.NextCursor != "" {
		enc := s.encodeCursor(res.NextCursor, res.Generation)
		next = &enc
	}
	rev := ""
	if st, err := s.svc.GetState(ctx, actor); err == nil && st != nil {
		rev = string(st.RuntimeRevision)
	}
	s.writeJSON(w, http.StatusOK, messageListJSON{
		Revision:        rev,
		StoreGeneration: res.Generation,
		Items:           items,
		NextCursor:      next,
	})
}

func listQueryFromRequest(r *http.Request) (model.ListQuery, error) {
	qs := r.URL.Query()
	q := model.ListQuery{
		Filter: model.MessageFilter{
			To:              qs.Get("to"),
			From:            qs.Get("from"),
			Subject:         qs.Get("subject"),
			SubjectContains: qs.Get("subjectContains"),
		},
		Cursor: qs.Get("cursor"),
	}
	if raw := qs.Get("unread"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return q, domainerr.ValidationFailed("invalid unread",
				domainerr.FieldViolation{Path: "unread", Code: "invalid_value", Message: "unread must be true or false"})
		}
		q.Filter.Unread = &b
	}
	after, err := parseOptionalTime(qs.Get("after"), "after")
	if err != nil {
		return q, err
	}
	q.Filter.After = after
	before, err := parseOptionalTime(qs.Get("before"), "before")
	if err != nil {
		return q, err
	}
	q.Filter.Before = before
	if raw := qs.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return q, domainerr.ValidationFailed("invalid limit",
				domainerr.FieldViolation{Path: "limit", Code: "invalid_value", Message: "limit must be a non-negative integer"})
		}
		q.Limit = n
	}
	return q, nil
}

func parseOptionalTime(raw, field string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, raw)
	}
	if err != nil {
		return time.Time{}, domainerr.ValidationFailed("invalid timestamp",
			domainerr.FieldViolation{Path: field, Code: "invalid_value", Message: field + " must be RFC3339"})
	}
	return t.UTC(), nil
}

func markReadFromQuery(r *http.Request) (bool, error) {
	raw := r.URL.Query().Get("markRead")
	if raw == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false, domainerr.ValidationFailed("invalid markRead",
			domainerr.FieldViolation{Path: "markRead", Code: "invalid_value", Message: "markRead must be true or false"})
	}
	return b, nil
}

func (s *Server) handleGetMessage(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id string) {
	mark, err := markReadFromQuery(r)
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	msg, err := s.svc.GetMessage(ctx, actor, id, mark)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromMessage(msg, false))
}

func (s *Server) handleRaw(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id string) {
	msg, err := s.svc.GetMessage(ctx, actor, id, false)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.writeBytes(w, http.StatusOK, "message/rfc822", msg.Raw)
	_ = r
}

func (s *Server) handleHTML(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id string) {
	msg, err := s.svc.GetMessage(ctx, actor, id, false)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeBytes(w, http.StatusOK, "text/html; charset=utf-8", []byte(msg.HTML))
	_ = r
}

func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id string) {
	in, err := s.deleteIn(w, r, instance)
	if err != nil {
		return
	}
	if err := s.svc.DeleteMessage(ctx, actor, id, in); err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleClear(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	in, err := s.deleteIn(w, r, instance)
	if err != nil {
		return
	}
	n, err := s.svc.ClearMessages(ctx, actor, in)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
}

func (s *Server) deleteIn(w http.ResponseWriter, r *http.Request, instance string) (app.DeleteIn, error) {
	var body deleteRequest
	if r.ContentLength != 0 && r.Header.Get("Content-Type") != "" {
		if !s.decodeJSONOptional(w, r, instance, &body) {
			return app.DeleteIn{}, domainerr.ValidationFailed("invalid body")
		}
	}
	if body.ExpectedStoreGeneration == nil {
		if raw := r.URL.Query().Get("expectedStoreGeneration"); raw != "" {
			n, err := parseStoreGeneration(raw)
			if err != nil {
				s.writeProblem(w, r, instance, err)
				return app.DeleteIn{}, err
			}
			body.ExpectedStoreGeneration = &n
		}
	}
	if body.ExpectedStoreGeneration == nil {
		if raw := strings.Trim(r.Header.Get(headerIfMatch), `"`); raw != "" && !strings.EqualFold(raw, "*") {
			n, err := parseStoreGeneration(raw)
			if err != nil {
				s.writeProblem(w, r, instance, err)
				return app.DeleteIn{}, err
			}
			body.ExpectedStoreGeneration = &n
		}
	}
	return app.DeleteIn{ExpectedStoreGeneration: body.ExpectedStoreGeneration}, nil
}

func parseStoreGeneration(raw string) (uint64, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0, domainerr.ValidationFailed("invalid expectedStoreGeneration",
			domainerr.FieldViolation{Path: "expectedStoreGeneration", Code: "invalid_value", Message: "expectedStoreGeneration must be an integer"})
	}
	return n, nil
}

func (s *Server) handleReadAll(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	n, err := s.svc.MarkAllRead(ctx, actor)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"updated": n})
	_ = r
}

func (s *Server) handleWait(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor) {
	var in waitRequest
	if !s.decodeJSON(w, r, instance, &in) {
		return
	}
	filter := model.MessageFilter{
		To: in.Filter.To, From: in.Filter.From, Subject: in.Filter.Subject,
		SubjectContains: in.Filter.SubjectContains, Unread: in.Filter.Unread,
	}
	after, err := parseOptionalTime(in.Filter.After, "filter.after")
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	filter.After = after
	before, err := parseOptionalTime(in.Filter.Before, "filter.before")
	if err != nil {
		s.writeProblem(w, r, instance, err)
		return
	}
	filter.Before = before

	var timeout time.Duration
	if in.Timeout != "" {
		d, err := time.ParseDuration(in.Timeout)
		if err != nil || d < 0 {
			s.writeProblem(w, r, instance, domainerr.ValidationFailed("invalid timeout",
				domainerr.FieldViolation{Path: "timeout", Code: "invalid_value", Message: "timeout must use Go duration syntax"}))
			return
		}
		timeout = d
	}
	msg, err := s.svc.Wait(ctx, actor, app.WaitIn{Filter: filter, Timeout: timeout})
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, fromMessage(msg, false))
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id string) {
	out, err := s.svc.Extract(ctx, actor, id)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	s.writeJSON(w, http.StatusOK, out)
	_ = r
}

func (s *Server) handleAttachment(w http.ResponseWriter, r *http.Request, instance string, ctx context.Context, actor app.Actor, id, attID string) {
	msg, err := s.svc.GetMessage(ctx, actor, id, false)
	if err != nil {
		s.writeProblem(w, r, instance, asDomain(err))
		return
	}
	var att *model.Attachment
	for i := range msg.Attachments {
		if msg.Attachments[i].ID == attID {
			att = &msg.Attachments[i]
			break
		}
	}
	if att == nil {
		s.writeProblem(w, r, instance, domainerr.NotFound("attachment not found"))
		return
	}
	name := att.Filename
	if name == "" {
		name = path.Base(att.ID)
	}
	ct := att.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeDownloadName(name)+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(att.Data)
	_ = r
}

func sanitizeDownloadName(name string) string {
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if name == "" {
		return "attachment"
	}
	return name
}
