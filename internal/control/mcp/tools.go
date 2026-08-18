package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"path"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/app"
	"github.com/hilather/go-lab-maildev/internal/buildinfo"
	"github.com/hilather/go-lab-maildev/internal/capabilities"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func (s *Server) registerTools() {
	addTool(s, "mail_version_get", versionDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		return fromVersion(buildinfo.Current()), nil
	})
	addTool(s, "mail_capabilities_get", capDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		return fromCapabilities(), nil
	})
	addTool(s, "mail_status_get", statusDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		return s.statusDTO(ctx, actor)
	})
	addTool(s, "mail_schema_get", schemaDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		_ = actor
		b, err := config.SchemaBytes()
		if err != nil {
			return nil, domainerr.Internal("schema unavailable")
		}
		var doc any
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, domainerr.Internal("internal error")
		}
		return doc, nil
	})
	addTool(s, "mail_state_get", stateGetDesc, false, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		v, err := s.svc.GetState(ctx, actor)
		if err != nil {
			return nil, err
		}
		return fromStateView(v)
	})
	addTool(s, "mail_state_validate", validateDesc, false, true, func(ctx context.Context, actor app.Actor, in validateIn) (any, error) {
		vin, err := in.toValidate()
		if err != nil {
			return nil, asDomain(err)
		}
		p, err := s.svc.Validate(ctx, actor, vin)
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "mail_change_plan", planDesc, false, true, func(ctx context.Context, actor app.Actor, in changeIn) (any, error) {
		p, err := s.svc.Plan(ctx, actor, in.toChange())
		if err != nil {
			return nil, err
		}
		return fromPlan(p), nil
	})
	addTool(s, "mail_change_apply", applyDesc, true, true, func(ctx context.Context, actor app.Actor, in changeIn) (any, error) {
		r, err := s.svc.Apply(ctx, actor, in.toChange())
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "mail_state_export", exportDesc, false, true, func(ctx context.Context, actor app.Actor, in exportIn) (any, error) {
		format := app.ExportYAML
		switch strings.ToLower(in.Format) {
		case "", "yaml", "yml":
		case "json":
			format = app.ExportJSON
		default:
			return nil, domainerr.ValidationFailed("unknown export format",
				domainerr.FieldViolation{Path: "format", Code: "invalid_value", Message: "format must be yaml or json"})
		}
		exp, err := s.svc.Export(ctx, actor, format)
		if err != nil {
			return nil, err
		}
		return fromExport(exp), nil
	})
	addTool(s, "mail_state_reset", resetDesc, true, false, func(ctx context.Context, actor app.Actor, in resetIn) (any, error) {
		r, err := s.svc.Reset(ctx, actor, app.ResetIn{Reason: in.Reason})
		if err != nil {
			return nil, err
		}
		return fromApply(r), nil
	})
	addTool(s, "mail_messages_list", messagesListDesc, false, true, func(ctx context.Context, actor app.Actor, in listIn) (any, error) {
		return s.listMessages(ctx, actor, in)
	})
	addTool(s, "mail_message_get", messageGetDesc, false, true, func(ctx context.Context, actor app.Actor, in getIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		msg, err := s.svc.GetMessage(ctx, actor, in.ID, in.MarkRead)
		if err != nil {
			return nil, err
		}
		return fromMessage(msg, false), nil
	})
	addTool(s, "mail_message_raw_get", messageRawDesc, false, true, func(ctx context.Context, actor app.Actor, in messageIDIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		msg, err := s.svc.GetMessage(ctx, actor, in.ID, false)
		if err != nil {
			return nil, err
		}
		return rawBodyJSON{ID: msg.ID, ContentType: "message/rfc822", Body: string(msg.Raw)}, nil
	})
	addTool(s, "mail_message_html_get", messageHTMLDesc, false, true, func(ctx context.Context, actor app.Actor, in messageIDIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		msg, err := s.svc.GetMessage(ctx, actor, in.ID, false)
		if err != nil {
			return nil, err
		}
		return htmlBodyJSON{ID: msg.ID, HTML: msg.HTML}, nil
	})
	addTool(s, "mail_message_delete", messageDeleteDesc, true, true, func(ctx context.Context, actor app.Actor, in deleteIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		if err := s.svc.DeleteMessage(ctx, actor, in.ID, app.DeleteIn{ExpectedStoreGeneration: in.ExpectedStoreGeneration}); err != nil {
			return nil, err
		}
		return map[string]any{"ok": true}, nil
	})
	addTool(s, "mail_messages_clear", messagesClearDesc, true, true, func(ctx context.Context, actor app.Actor, in clearIn) (any, error) {
		n, err := s.svc.ClearMessages(ctx, actor, app.DeleteIn{ExpectedStoreGeneration: in.ExpectedStoreGeneration})
		if err != nil {
			return nil, err
		}
		return countJSON{Deleted: n}, nil
	})
	addTool(s, "mail_messages_read_all", messagesReadAllDesc, true, true, func(ctx context.Context, actor app.Actor, _ emptyIn) (any, error) {
		n, err := s.svc.MarkAllRead(ctx, actor)
		if err != nil {
			return nil, err
		}
		return countJSON{Updated: n}, nil
	})
	addTool(s, "mail_messages_wait", messagesWaitDesc, false, true, func(ctx context.Context, actor app.Actor, in waitIn) (any, error) {
		win, err := in.toWait()
		if err != nil {
			return nil, err
		}
		msg, err := s.svc.Wait(ctx, actor, win)
		if err != nil {
			return nil, err
		}
		return fromMessage(msg, false), nil
	})
	addTool(s, "mail_message_extract", messageExtractDesc, false, true, func(ctx context.Context, actor app.Actor, in messageIDIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		return s.svc.Extract(ctx, actor, in.ID)
	})
	addTool(s, "mail_attachment_get", attachmentGetDesc, false, true, func(ctx context.Context, actor app.Actor, in attachmentIn) (any, error) {
		if in.ID == "" || in.AttID == "" {
			return nil, domainerr.ValidationFailed("id and attId are required",
				domainerr.FieldViolation{Path: "attId", Code: "required", Message: "id and attId are required"})
		}
		msg, err := s.svc.GetMessage(ctx, actor, in.ID, false)
		if err != nil {
			return nil, err
		}
		var att *model.Attachment
		for i := range msg.Attachments {
			if msg.Attachments[i].ID == in.AttID {
				att = &msg.Attachments[i]
				break
			}
		}
		if att == nil {
			return nil, domainerr.NotFound("attachment not found")
		}
		name := att.Filename
		if name == "" {
			name = path.Base(att.ID)
		}
		ct := att.ContentType
		if ct == "" {
			ct = "application/octet-stream"
		}
		return attachmentGetJSON{
			attachmentJSON: attachmentJSON{
				ID: att.ID, Filename: name, ContentType: ct,
				ContentID: att.ContentID, Disposition: att.Disposition, Size: att.Size, Checksum: att.Checksum,
			},
			Data: base64.StdEncoding.EncodeToString(att.Data),
		}, nil
	})
	addTool(s, "mail_audit_query", auditQueryDesc, false, true, func(ctx context.Context, actor app.Actor, in auditQueryIn) (any, error) {
		list, err := s.svc.QueryAudit(ctx, actor, app.AuditQuery{Limit: in.Limit})
		if err != nil {
			return nil, err
		}
		return fromAuditList(list), nil
	})
	addTool(s, "mail_audit_get", auditGetDesc, false, true, func(ctx context.Context, actor app.Actor, in idIn) (any, error) {
		if in.ID == "" {
			return nil, domainerr.ValidationFailed("id is required",
				domainerr.FieldViolation{Path: "id", Code: "required", Message: "id is required"})
		}
		return s.svc.GetAudit(ctx, actor, in.ID)
	})
}

func (s *Server) statusDTO(ctx context.Context, actor app.Actor) (statusJSON, error) {
	st, err := s.svc.Status(ctx, actor)
	if err != nil {
		return statusJSON{}, err
	}
	view, err := s.svc.GetState(ctx, actor)
	if err != nil {
		return statusJSON{}, err
	}
	return fromStatus(st, view)
}

func (s *Server) listMessages(ctx context.Context, actor app.Actor, in listIn) (messageListJSON, error) {
	filter, err := in.filter()
	if err != nil {
		return messageListJSON{}, err
	}
	q := model.ListQuery{Filter: filter, Cursor: in.Cursor, Limit: in.Limit}
	rawCursor := q.Cursor
	var cursorGen uint64
	if rawCursor != "" {
		id, gen, err := s.decodeCursor(rawCursor)
		if err != nil {
			return messageListJSON{}, err
		}
		q.Cursor = id
		cursorGen = gen
	}
	res, err := s.svc.ListMessages(ctx, actor, q)
	if err != nil {
		return messageListJSON{}, err
	}
	if rawCursor != "" && cursorGen != res.Generation {
		return messageListJSON{}, domainerr.CursorStale("list cursor is stale; restart the list")
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
	return messageListJSON{
		Revision:        rev,
		StoreGeneration: res.Generation,
		Items:           items,
		NextCursor:      next,
	}, nil
}

func addTool[In any](s *Server, name, desc string, mutating, idempotent bool, h func(context.Context, app.Actor, In) (any, error)) {
	caps := capabilities.LookupTool(name)
	title := name
	if len(caps) > 0 && caps[0].Title != "" {
		title = caps[0].Title
		if desc == "" {
			desc = caps[0].Description
		}
	}
	readOnly := !mutating
	ann := &sdk.ToolAnnotations{
		Title:           title,
		ReadOnlyHint:    readOnly,
		IdempotentHint:  idempotent,
		DestructiveHint: boolPtr(mutating && !idempotent),
		OpenWorldHint:   boolPtr(false),
	}
	sdk.AddTool(s.sdk, &sdk.Tool{
		Name:        name,
		Title:       title,
		Description: desc,
		Annotations: ann,
	}, func(ctx context.Context, _ *sdk.CallToolRequest, in In) (*sdk.CallToolResult, any, error) {
		if err := ctx.Err(); err != nil {
			return toolErrorResult(canceledError(err)), nil, nil
		}
		actor := actorFrom(ctx)
		if err := s.authorizeTool(actor, name); err != nil {
			return toolErrorResult(err), nil, nil
		}
		out, err := h(ctx, actor, in)
		if err != nil {
			return toolErrorResult(err), nil, nil
		}
		structured, err := asStructured(out)
		if err != nil {
			return nil, nil, rpcError(domainerr.Internal("internal error"))
		}
		return nil, structured, nil
	})
}

func boolPtr(v bool) *bool { return &v }

func canceledError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return domainerr.Timeout("request deadline exceeded")
	}
	return domainerr.Internal("request canceled")
}

const (
	versionDesc         = "Read-only. Build and protocol versions (MCP " + ProtocolVersion + ")."
	capDesc             = "Read-only. Capability list and protocol metadata."
	statusDesc          = "Read-only. Listeners, store stats, and revisions."
	schemaDesc          = "Read-only. Published v1alpha1 config JSON Schema."
	stateGetDesc        = "Read-only. Redacted spec plus revision metadata."
	validateDesc        = "Read-only dry-run. Validate a candidate document and/or operations without writing."
	planDesc            = "Read-only dry-run. Plan operations against the active snapshot."
	applyDesc           = "State-changing. Apply operations with expectedRevision. High-impact."
	exportDesc          = "Read-only. Canonical desired-state export plus drift material."
	resetDesc           = "State-changing, high-impact. Reread the bootstrap mount, wipe the inbox, and swap. Never writes the file."
	messagesListDesc    = "Read-only. Cursor-paginated inbox list. HMAC cursors bind storeGeneration."
	messageGetDesc      = "Read-only. Full message. markRead defaults to false."
	messageRawDesc      = "Read-only. RFC 822 source (message/rfc822)."
	messageHTMLDesc     = "Read-only. Raw HTML body with no CSP."
	messageDeleteDesc   = "State-changing. Delete one message by id."
	messagesClearDesc   = "State-changing. Delete every message. Does not bump epoch."
	messagesReadAllDesc = "State-changing. Set every read bit. Does not bump storeGeneration."
	messagesWaitDesc    = "Read-only. Block until a matching message arrives or the timeout fires."
	messageExtractDesc  = "Read-only. Extract URLs and OTP-like tokens using the frozen RE2 set."
	attachmentGetDesc   = "Read-only. Download one attachment as base64."
	auditQueryDesc      = "Read-only. Query recent in-memory audit events."
	auditGetDesc        = "Read-only. Get one audit event by id."
)
