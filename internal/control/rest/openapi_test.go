package rest

import (
	"encoding/json"
	"testing"

	"github.com/hilather/go-lab-maildev/internal/capabilities"
)

func TestRenderOpenAPI(t *testing.T) {
	b, err := RenderOpenAPI()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["openapi"] != OpenAPIVersion {
		t.Fatalf("openapi=%v", doc["openapi"])
	}
	paths, _ := doc["paths"].(map[string]any)
	if _, ok := paths["/v1/messages:wait"]; !ok {
		t.Fatal("missing wait path")
	}
	if _, ok := paths["/v1/messages/{id}/preview"]; !ok {
		t.Fatal("missing preview path")
	}
	for _, c := range capabilities.All() {
		for _, bind := range c.REST {
			if _, ok := paths[bind.Path]; !ok {
				t.Errorf("openapi missing %s", bind.Path)
			}
		}
	}
}
