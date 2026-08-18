package mcp

import (
	"context"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerResources() {
	h := s.readResource
	s.sdk.AddResource(&sdk.Resource{
		URI: "labmail://capabilities", Name: "capabilities",
		Description: "Capability list and protocol metadata.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labmail://status", Name: "status",
		Description: "Listeners, store stats, and revisions.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labmail://schema/config", Name: "schema-config",
		Description: "Published v1alpha1 config JSON Schema.",
		MIMEType:    "application/schema+json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labmail://state", Name: "state",
		Description: "Redacted spec plus revision metadata (same as GET /v1/state).",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labmail://messages", Name: "messages",
		Description: "Cursor-paginated inbox list. subscriptions/listen notifies URI only.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResourceTemplate(&sdk.ResourceTemplate{
		URITemplate: "labmail://messages/{id}", Name: "message",
		Description: "One message by id. markRead is false.",
		MIMEType:    "application/json",
	}, h)
	s.sdk.AddResource(&sdk.Resource{
		URI: "labmail://audit/recent", Name: "audit-recent",
		Description: "Recent in-memory audit events.",
		MIMEType:    "application/json",
	}, h)
}

func (s *Server) readResource(ctx context.Context, req *sdk.ReadResourceRequest) (*sdk.ReadResourceResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, rpcError(domainerr.Internal("request canceled"))
	}
	actor := s.actorFrom(ctx)
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	if err := s.authorizeResource(actor, uri); err != nil {
		return nil, rpcError(err)
	}
	body, mime, err := s.resourceBody(ctx, actor, uri)
	if err != nil {
		return nil, rpcError(err)
	}
	return &sdk.ReadResourceResult{
		Contents: []*sdk.ResourceContents{{
			URI:      uri,
			MIMEType: mime,
			Text:     string(body),
		}},
	}, nil
}

func (s *Server) resourceBody(ctx context.Context, actor app.Actor, uri string) ([]byte, string, error) {
	switch {
	case uri == "labmail://capabilities":
		b, err := marshalAPI(fromCapabilities())
		return b, "application/json", err
	case uri == "labmail://status":
		st, err := s.statusDTO(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(st)
		return b, "application/json", err
	case uri == "labmail://schema/config":
		b, err := config.SchemaBytes()
		return b, "application/schema+json", err
	case uri == "labmail://state":
		v, err := s.svc.GetState(ctx, actor)
		if err != nil {
			return nil, "", err
		}
		view, err := fromStateView(v)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(view)
		return b, "application/json", err
	case uri == "labmail://messages":
		list, err := s.listMessages(ctx, actor, listIn{})
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(list)
		return b, "application/json", err
	case strings.HasPrefix(uri, "labmail://messages/"):
		id := strings.TrimPrefix(uri, "labmail://messages/")
		if id == "" || strings.Contains(id, "/") {
			return nil, "", domainerr.NotFound("resource not found")
		}
		msg, err := s.svc.GetMessage(ctx, actor, id, false)
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromMessage(msg, false))
		return b, "application/json", err
	case uri == "labmail://audit/recent":
		v, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{})
		if err != nil {
			return nil, "", err
		}
		b, err := marshalAPI(fromAuditList(v))
		return b, "application/json", err
	default:
		return nil, "", domainerr.NotFound("resource not found")
	}
}
