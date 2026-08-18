package model

import "testing"

func TestAPIIdentity(t *testing.T) {
	if APIVersionV1Alpha1 != "labmail.dev/v1alpha1" {
		t.Fatalf("apiVersion=%q", APIVersionV1Alpha1)
	}
	if KindLabMail != "LabMail" {
		t.Fatalf("kind=%q", KindLabMail)
	}
	if RevisionPrefix != "sha256:" {
		t.Fatalf("prefix=%q", RevisionPrefix)
	}
}

func TestKnownEnums(t *testing.T) {
	if !KnownScope(ScopeMailRead) || KnownScope("dns.read") {
		t.Fatal("scope set")
	}
	if !KnownRole(RoleAdministrator) || KnownRole("root") {
		t.Fatal("role set")
	}
	if !KnownOp(OpReplaceStoreCaps) || KnownOp("addZone") {
		t.Fatal("op set")
	}
}
