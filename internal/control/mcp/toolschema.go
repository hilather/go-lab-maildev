package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hilather/go-lab-maildev/internal/config"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// Shadow inputs keep []model.Operation so jsonschema.For does not infer
// json.RawMessage operations as a string. The handler types stay raw so
// CoerceWireChange sees the original numbers.
type changeInSchema struct {
	ExpectedRevision string            `json:"expectedRevision,omitempty"`
	IdempotencyKey   string            `json:"idempotencyKey,omitempty"`
	Reason           string            `json:"reason,omitempty"`
	Force            bool              `json:"force,omitempty"`
	Operations       []model.Operation `json:"operations,omitempty"`
}

type validateInSchema struct {
	State      json.RawMessage   `json:"state,omitempty"`
	Operations []model.Operation `json:"operations,omitempty"`
}

const (
	durationWireDesc = "Canonical form is a Go duration (30s). A bare number is rejected."
	sizeWireDesc     = "Canonical form is an IEC size (10MiB). A bare number is rejected."
)

var (
	changeToolInputSchema   = mustInputSchema[changeInSchema]("mail_change")
	validateToolInputSchema = validateInputSchema()
)

func validateInputSchema() *jsonschema.Schema {
	s := mustInputSchema[validateInSchema]("mail_state_validate")
	if s.Properties == nil {
		s.Properties = map[string]*jsonschema.Schema{}
	}
	// json.RawMessage infers as a byte array. Candidate documents are JSON
	// objects; DecodeJSON checks the document after the schema accepts it.
	s.Properties["state"] = &jsonschema.Schema{Types: []string{"null", "object"}}
	return s
}

func mustInputSchema[T any](name string) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("mcp input schema %s: %v", name, err))
	}
	if s == nil || s.Type != "object" {
		panic(fmt.Sprintf("mcp input schema %s: root type must be object", name))
	}
	rewriteWireSchemas(s)
	return s
}

func rewriteWireSchemas(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	for name, prop := range s.Properties {
		if prop == nil {
			continue
		}
		switch {
		case isDurationField(name):
			relaxWireSchema(prop, durationWireDesc)
		case isSizeField(name):
			relaxWireSchema(prop, sizeWireDesc)
		}
		rewriteWireSchemas(prop)
	}
	rewriteWireSchemas(s.Items)
	for _, sub := range s.PrefixItems {
		rewriteWireSchemas(sub)
	}
	for _, sub := range s.AllOf {
		rewriteWireSchemas(sub)
	}
	for _, sub := range s.AnyOf {
		rewriteWireSchemas(sub)
	}
	for _, sub := range s.OneOf {
		rewriteWireSchemas(sub)
	}
	for _, sub := range s.Defs {
		rewriteWireSchemas(sub)
	}
}

func isDurationField(name string) bool {
	_, ok := config.CanonicalDurationField(name)
	return ok
}

func isSizeField(name string) bool {
	_, ok := config.CanonicalSizeField(name)
	return ok
}

func relaxWireSchema(s *jsonschema.Schema, desc string) {
	s.Type = ""
	s.Types = []string{"string", "integer"}
	s.Minimum = nil
	s.Maximum = nil
	s.ExclusiveMinimum = nil
	s.ExclusiveMaximum = nil
	s.Description = desc
}
