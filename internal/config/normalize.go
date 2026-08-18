package config

import (
	"encoding/json"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

// Normalize returns a copy of st with nil slices materialized. Numeric and
// bool defaults are applied at decode time so explicit zeros stay visible.
func Normalize(st *model.State) (*model.State, error) {
	if st == nil {
		return nil, domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: violationRequired, Message: "state is nil"})
	}
	out, err := cloneState(st)
	if err != nil {
		return nil, err
	}
	materializeDefaults(&out.Spec)
	return out, nil
}

func cloneState(st *model.State) (*model.State, error) {
	b, err := json.Marshal(st)
	if err != nil {
		return nil, domainerr.Internal("clone marshal: " + err.Error())
	}
	var out model.State
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, domainerr.Internal("clone unmarshal: " + err.Error())
	}
	return &out, nil
}

func materializeDefaults(sp *model.Spec) {
	if sp.SMTP.HideExtensions == nil {
		sp.SMTP.HideExtensions = []string{}
	}
	if sp.Management.Auth.Tokens == nil {
		sp.Management.Auth.Tokens = []model.TokenSpec{}
	}
	if sp.Management.OriginAllowlist == nil {
		sp.Management.OriginAllowlist = []string{}
	}
}
