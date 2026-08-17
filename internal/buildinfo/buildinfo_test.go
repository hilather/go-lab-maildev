package buildinfo

import (
	"strings"
	"testing"
)

func TestCurrentProtocols(t *testing.T) {
	info := Current()
	if info.Version == "" {
		t.Fatal("Version is empty")
	}
	if info.Commit == "" {
		t.Fatal("Commit is empty")
	}
	if info.BuildTime == "" {
		t.Fatal("BuildTime is empty")
	}
	if info.Protocols.ConfigAPI != ConfigAPIVersion {
		t.Fatalf("ConfigAPI = %q, want %q", info.Protocols.ConfigAPI, ConfigAPIVersion)
	}
	if info.Protocols.REST != RESTPrefix {
		t.Fatalf("REST = %q, want %q", info.Protocols.REST, RESTPrefix)
	}
	if info.Protocols.MCP != MCPProtocol {
		t.Fatalf("MCP = %q, want %q", info.Protocols.MCP, MCPProtocol)
	}
	if info.Protocols.Compat != CompatPrefix {
		t.Fatalf("Compat = %q, want %q", info.Protocols.Compat, CompatPrefix)
	}
	s := info.String()
	if !strings.Contains(s, "labmail") || !strings.Contains(s, MCPProtocol) {
		t.Fatalf("String() = %q, missing expected tokens", s)
	}
}

func TestInfoStringEmpty(t *testing.T) {
	var info Info
	if info.String() == "" {
		t.Fatal("empty Info.String() is empty")
	}
}

func FuzzInfoString(f *testing.F) {
	f.Add("dev", "abc123", "2026-08-17T00:00:00Z")
	f.Add("", "", "")
	f.Fuzz(func(t *testing.T, version, commit, built string) {
		info := Info{
			Version:   version,
			Commit:    commit,
			BuildTime: built,
			Protocols: Protocols{
				ConfigAPI: ConfigAPIVersion,
				REST:      RESTPrefix,
				MCP:       MCPProtocol,
				Compat:    CompatPrefix,
			},
		}
		if info.String() == "" {
			t.Fatal("String() returned empty")
		}
	})
}
