package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/model"
)

func TestPreviewCSPAndCID(t *testing.T) {
	s, svc := newTestServer(t)
	msg := &model.Message{
		HTML: `<html><img src="cid:pic@lab"></html>`,
		Attachments: []model.Attachment{{
			ContentID:   "<pic@lab>",
			ContentType: "image/png",
			Data:        []byte{0x89, 0x50, 0x4e, 0x47},
		}},
	}
	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), msg)
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/messages/"+res.ID+"/preview", "")
	requireStatus(t, got, http.StatusOK)
	if got.Header().Get("Content-Security-Policy") != previewCSP {
		t.Fatalf("csp=%q", got.Header().Get("Content-Security-Policy"))
	}
	if got.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if got.Header().Get("Content-Disposition") != "inline" {
		t.Fatal("disposition")
	}
	if !strings.Contains(got.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("ct=%s", got.Header().Get("Content-Type"))
	}
	if strings.Contains(got.Body.String(), "cid:") {
		t.Fatalf("cid not rewritten: %s", got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "data:image/png;base64,") {
		t.Fatalf("missing data url: %s", got.Body.String())
	}
	if strings.Contains(got.Body.String(), "/attachments/") {
		t.Fatal("must not rewrite cid to HTTP attachment paths")
	}
}

func TestPreviewSanitizesContentType(t *testing.T) {
	s, svc := newTestServer(t)
	msg := &model.Message{
		HTML: `<html><img src="cid:x"></html>`,
		Attachments: []model.Attachment{{
			ContentID:   "<x>",
			ContentType: `foo" onerror="alert(1)`,
			Data:        []byte{1, 2, 3},
		}},
	}
	res, err := svc.Inbox().Insert(context.Background(), svc.Inbox().Epoch(), msg)
	if err != nil {
		t.Fatal(err)
	}
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/messages/"+res.ID+"/preview", "")
	requireStatus(t, got, http.StatusOK)
	if strings.Contains(got.Body.String(), "onerror") || strings.Contains(got.Body.String(), `foo"`) {
		t.Fatalf("unsafe type leaked: %s", got.Body.String())
	}
	if !strings.Contains(got.Body.String(), "data:application/octet-stream;base64,") {
		t.Fatalf("expected octet-stream fallback: %s", got.Body.String())
	}
}

func TestHTMLHasNoCSP(t *testing.T) {
	s, svc := newTestServer(t)
	id := insertMail(t, svc, "h", "<b>x</b>")
	got := doReq(t, s.Handler(), http.MethodGet, "/v1/messages/"+id+"/html", "")
	requireStatus(t, got, http.StatusOK)
	if got.Header().Get("Content-Security-Policy") != "" {
		t.Fatal("raw html must not set CSP")
	}
}
