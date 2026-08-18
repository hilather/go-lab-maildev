package capabilities

import (
	"testing"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

func TestCatalogStructure(t *testing.T) {
	if err := ValidateCatalog(); err != nil {
		t.Fatal(err)
	}
	all := All()
	if len(all) != TableRowCount {
		t.Fatalf("len(All())=%d want %d", len(all), TableRowCount)
	}
	if _, ok := Lookup(HealthLive); !ok {
		t.Fatal("Lookup(health.live) missing")
	}
	if _, ok := Lookup(ID("not-a-capability")); ok {
		t.Fatal("Lookup unknown succeeded")
	}
	live := MustLookup(HealthLive)
	if !live.RESTOnly || live.MCP != nil {
		t.Fatalf("health.live must be REST-only: %+v", live)
	}
	ready := MustLookup(HealthReady)
	if !ready.RESTOnly {
		t.Fatal("health.ready must be REST-only")
	}
	prev := MustLookup(MessagesPreview)
	if !prev.RESTOnly {
		t.Fatal("messages.preview must be REST-only")
	}
	if !SessionCapability(SessionCreate) || SessionCapability(MessagesList) {
		t.Fatal("SessionCapability")
	}
}

func TestFrozenIDsStable(t *testing.T) {
	want := []ID{
		HealthLive, HealthReady, VersionGet, CapabilitiesGet, StatusGet, SchemaGet,
		StateGet, StateValidate, StateExport, StateReset, ChangesPlan, ChangesApply,
		SessionCreate, SessionDelete, SessionGet, EventsStream,
		MessagesList, MessagesGet, MessagesRaw, MessagesHTML, MessagesPreview,
		MessagesDelete, MessagesClear, MessagesReadAll, MessagesWait, MessagesExtract,
		AttachmentsGet, AuditList, AuditGet, MetricsGet,
	}
	got := All()
	if len(got) != len(want) {
		t.Fatalf("catalog ids=%d frozen=%d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Fatalf("row %d id=%q want %q", i, got[i].ID, want[i])
		}
	}
}

func TestLookupRESTAndTool(t *testing.T) {
	c, ok := LookupREST("GET", "/v1/state")
	if !ok || c.ID != StateGet {
		t.Fatalf("LookupREST GET /v1/state = %+v ok=%v", c, ok)
	}
	tools := LookupTool("mail_change_apply")
	if len(tools) != 1 || tools[0].ID != ChangesApply {
		t.Fatalf("LookupTool apply = %+v", tools)
	}
	if _, ok := LookupResource("labmail://status"); !ok {
		t.Fatal("missing labmail://status")
	}
	if _, ok := LookupREST("GET", "/v1/nope"); ok {
		t.Fatal("unexpected REST hit")
	}
	wait, ok := LookupREST("POST", "/v1/messages:wait")
	if !ok || wait.ID != MessagesWait {
		t.Fatalf("wait=%+v ok=%v", wait, ok)
	}
}

func TestHealthHasNoTools(t *testing.T) {
	for _, id := range []ID{HealthLive, HealthReady, MessagesPreview, MetricsGet} {
		c := MustLookup(id)
		if !c.RESTOnly {
			t.Errorf("%s RESTOnly=false", id)
		}
		if c.MCP != nil {
			t.Errorf("%s has MCP binding %+v", id, c.MCP)
		}
	}
}

func TestNativeGetDefaultNotWrite(t *testing.T) {
	c := MustLookup(MessagesGet)
	if c.Mutating {
		t.Fatal("messages.get must not be mutating; markRead default is false")
	}
}

func TestProblemFromDomainCodes(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   domainerr.Code
		typ    string
	}{
		{domainerr.CursorStale("stale"), 400, domainerr.CodeCursorStale, "urn:labmail:error:cursor-stale"},
		{domainerr.StoreOverNewCap("over"), 400, domainerr.CodeStoreOverNewCap, "urn:labmail:error:store-over-new-cap"},
		{domainerr.ValidationFailed("bad"), 400, domainerr.CodeValidationFailed, "urn:labmail:error:validation-failed"},
		{domainerr.NotFound("gone"), 404, domainerr.CodeNotFound, "urn:labmail:error:not-found"},
		{domainerr.ReceiveOnly("no"), 403, domainerr.CodeReceiveOnly, "urn:labmail:error:receive-only"},
		{domainerr.Timeout("wait"), 504, domainerr.CodeTimeout, "urn:labmail:error:timeout"},
	}
	for _, tc := range cases {
		p := ProblemFrom(tc.err, "urn:labmail:request:01TEST")
		if p.Status != tc.status || p.Code != tc.code || p.Type != tc.typ {
			t.Fatalf("ProblemFrom(%v)=status=%d code=%s type=%s", tc.err, p.Status, p.Code, p.Type)
		}
		if p.Code == domainerr.CodeValidationFailed && tc.code == domainerr.CodeCursorStale {
			t.Fatal("cursor_stale must not wrap as validation_failed")
		}
	}
}

func TestProblemFromUnknownIsInternal(t *testing.T) {
	p := ProblemFrom(assertErr("boom"), "")
	if p.Code != domainerr.CodeInternalError || p.Detail == "boom" {
		t.Fatalf("unknown error leaked: %+v", p)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
