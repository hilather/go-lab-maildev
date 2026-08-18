package app

import (
	"strconv"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

func applyOperations(st *model.State, ops []model.Operation) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: "required", Message: "state is nil"})
	}
	for i, op := range ops {
		if err := applyOne(st, op, i); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(st *model.State, op model.Operation, i int) error {
	path := "operations[" + strconv.Itoa(i) + "]"
	switch op.Op {
	case model.OpReplaceSMTPAuth:
		if op.Auth == nil {
			return domainerr.ValidationFailed("missing auth",
				domainerr.FieldViolation{Path: path + ".auth", Code: "required", Message: "replaceSMTPAuth requires auth"})
		}
		st.Spec.SMTP.Auth = *op.Auth
	case model.OpReplaceStoreCaps:
		if op.Store == nil {
			return domainerr.ValidationFailed("missing store",
				domainerr.FieldViolation{Path: path + ".store", Code: "required", Message: "replaceStoreCaps requires store"})
		}
		st.Spec.Store.MaxMessages = op.Store.MaxMessages
		st.Spec.Store.MaxBytes = op.Store.MaxBytes
		st.Spec.Store.FullPolicy = op.Store.FullPolicy
	case model.OpReplaceHideExtensions:
		if op.HideExtensions == nil {
			st.Spec.SMTP.HideExtensions = []string{}
		} else {
			st.Spec.SMTP.HideExtensions = append([]string(nil), op.HideExtensions...)
		}
	case model.OpReplaceAdmission:
		if op.Admission == nil {
			return domainerr.ValidationFailed("missing admission",
				domainerr.FieldViolation{Path: path + ".admission", Code: "required", Message: "replaceAdmission requires admission"})
		}
		st.Spec.SMTP.Admission = *op.Admission
	default:
		return domainerr.ValidationFailed("unknown operation",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "unknown op"})
	}
	return nil
}

func hasReplaceStoreCaps(ops []model.Operation) *model.StoreCaps {
	for i := range ops {
		if ops[i].Op == model.OpReplaceStoreCaps && ops[i].Store != nil {
			return ops[i].Store
		}
	}
	return nil
}
