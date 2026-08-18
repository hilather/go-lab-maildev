package mimeparse

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message/mail"

	// Register charset decoders used by go-message.
	_ "github.com/emersion/go-message/charset"

	"github.com/hilather/go-lab-maildev/internal/model"
)

// Parse extracts model fields from RFC 5322 bytes. Raw is always preserved.
func Parse(raw []byte) (msg *model.Message) {
	cp := append([]byte(nil), raw...)
	msg = &model.Message{
		Raw:      cp,
		Size:     len(cp),
		Priority: "normal",
	}
	defer func() {
		if rec := recover(); rec != nil {
			appendWarning(msg, fmt.Sprintf("panic: %v", rec))
		}
	}()
	if len(bytes.TrimSpace(cp)) == 0 {
		msg.ParseWarning = "empty message"
		return msg
	}

	mr, err := mail.CreateReader(bytes.NewReader(cp))
	if err != nil {
		msg.ParseWarning = err.Error()
		return msg
	}
	defer func() { _ = mr.Close() }()

	fillEnvelopeHeaders(msg, mr.Header)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			appendWarning(msg, err.Error())
			break
		}
		if err := consumePart(msg, p); err != nil {
			appendWarning(msg, err.Error())
		}
	}
	return msg
}

func fillEnvelopeHeaders(msg *model.Message, h mail.Header) {
	fields := h.Fields()
	for fields.Next() {
		msg.Headers = append(msg.Headers, model.Header{
			Name:  fields.Key(),
			Value: fields.Value(),
		})
	}
	if subj, err := h.Subject(); err != nil {
		appendWarning(msg, err.Error())
		if subj != "" {
			msg.Subject = subj
		}
	} else {
		msg.Subject = subj
	}
	msg.From = addressList(h, "From")
	msg.To = addressList(h, "To")
	msg.Cc = addressList(h, "Cc")
	msg.Bcc = addressList(h, "Bcc")
	msg.ReplyTo = addressList(h, "Reply-To")
	if id, err := h.MessageID(); err != nil {
		appendWarning(msg, err.Error())
	} else {
		msg.MessageID = id
	}
	if ids, err := h.MsgIDList("In-Reply-To"); err != nil {
		appendWarning(msg, err.Error())
	} else if len(ids) > 0 {
		msg.InReplyTo = ids[0]
	}
	if dt, err := h.Date(); err != nil {
		appendWarning(msg, err.Error())
	} else {
		msg.Date = dt
	}
	msg.Priority = priorityOf(h)
}

func addressList(h mail.Header, key string) []model.Address {
	list, err := h.AddressList(key)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]model.Address, 0, len(list))
	for _, a := range list {
		if a == nil {
			continue
		}
		out = append(out, model.Address{Name: a.Name, Address: a.Address})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func priorityOf(h mail.Header) string {
	imp := strings.ToLower(strings.TrimSpace(h.Get("Importance")))
	switch imp {
	case "high", "highest":
		return "high"
	case "low", "lowest":
		return "low"
	}
	xp := strings.TrimSpace(h.Get("X-Priority"))
	if xp == "" {
		return "normal"
	}
	switch xp[0] {
	case '1', '2':
		return "high"
	case '4', '5':
		return "low"
	default:
		return "normal"
	}
}

func consumePart(msg *model.Message, p *mail.Part) error {
	if p == nil {
		return nil
	}
	body, err := io.ReadAll(p.Body)
	if err != nil {
		return err
	}
	switch h := p.Header.(type) {
	case *mail.InlineHeader:
		return consumeInline(msg, h, body)
	case *mail.AttachmentHeader:
		return consumeAttachmentHeader(msg, h, body)
	default:
		addAttachment(msg, "attachment", "application/octet-stream", "", "attachment", body)
		return nil
	}
}

func consumeInline(msg *model.Message, h *mail.InlineHeader, body []byte) error {
	ctype, _, err := h.ContentType()
	if err != nil {
		appendWarning(msg, err.Error())
	}
	if ctype == "" {
		ctype = "text/plain"
	}
	disp, params, _ := h.ContentDisposition()
	filename := ""
	if params != nil {
		filename = params["filename"]
	}
	cid := stripAngles(h.Get("Content-Id"))
	media := strings.ToLower(ctype)
	asAttach := disp == "attachment" || filename != "" || cid != ""
	if strings.HasPrefix(media, "text/plain") && !asAttach {
		msg.Text = string(body)
		return nil
	}
	if strings.HasPrefix(media, "text/html") {
		msg.HTML = string(body)
		if !asAttach {
			return nil
		}
		if disp == "" {
			disp = "inline"
		}
		addAttachment(msg, filename, media, cid, disp, body)
		return nil
	}
	if strings.HasPrefix(media, "text/plain") {
		if msg.Text == "" {
			msg.Text = string(body)
		}
		if asAttach {
			if disp == "" {
				disp = "inline"
			}
			addAttachment(msg, filename, media, cid, disp, body)
		}
		return nil
	}
	if disp == "" {
		disp = "inline"
	}
	addAttachment(msg, filename, media, cid, disp, body)
	return nil
}

func consumeAttachmentHeader(msg *model.Message, h *mail.AttachmentHeader, body []byte) error {
	ctype, _, err := h.ContentType()
	if err != nil {
		appendWarning(msg, err.Error())
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	name, err := h.Filename()
	if err != nil {
		appendWarning(msg, err.Error())
	}
	disp, _, _ := h.ContentDisposition()
	if disp == "" {
		disp = "attachment"
	}
	addAttachment(msg, name, strings.ToLower(ctype), stripAngles(h.Get("Content-Id")), disp, body)
	return nil
}

func addAttachment(msg *model.Message, filename, contentType, contentID, disposition string, body []byte) {
	if disposition != "inline" {
		disposition = "attachment"
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	sum := sha256.Sum256(body)
	msg.Attachments = append(msg.Attachments, model.Attachment{
		Filename:    SanitizeFilename(filename),
		ContentType: contentType,
		ContentID:   contentID,
		Disposition: disposition,
		Size:        len(body),
		Checksum:    hex.EncodeToString(sum[:]),
		Data:        append([]byte(nil), body...),
	})
}

func stripAngles(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	return s
}

func appendWarning(msg *model.Message, w string) {
	w = strings.TrimSpace(w)
	if msg == nil || w == "" {
		return
	}
	if msg.ParseWarning == "" {
		msg.ParseWarning = w
		return
	}
	if strings.Contains(msg.ParseWarning, w) {
		return
	}
	msg.ParseWarning = msg.ParseWarning + "; " + w
}
