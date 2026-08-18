package model

// Closed plan/apply verbs. Fine-grained record CRUD is not in 1.0.
const (
	OpReplaceSMTPAuth       = "replaceSMTPAuth"
	OpReplaceStoreCaps      = "replaceStoreCaps"
	OpReplaceHideExtensions = "replaceHideExtensions"
	OpReplaceAdmission      = "replaceAdmission"
)

// ChangeSet is the LabDNS-shaped plan/apply envelope.
type ChangeSet struct {
	ExpectedRevision string      `json:"expectedRevision"`
	IdempotencyKey   string      `json:"idempotencyKey"`
	Reason           string      `json:"reason"`
	Force            bool        `json:"force"`
	Operations       []Operation `json:"operations"`
}

// Operation is one typed config mutation.
type Operation struct {
	Op             string         `json:"op"`
	Auth           *SMTPAuthSpec  `json:"auth,omitempty"`
	Store          *StoreCaps     `json:"store,omitempty"`
	HideExtensions []string       `json:"hideExtensions,omitempty"`
	Admission      *AdmissionSpec `json:"admission,omitempty"`
}

// StoreCaps is the replaceStoreCaps body.
type StoreCaps struct {
	MaxMessages int    `json:"maxMessages"`
	MaxBytes    int64  `json:"maxBytes"`
	FullPolicy  string `json:"fullPolicy"`
}

// KnownOp reports whether op is a v1alpha1 plan/apply verb.
func KnownOp(op string) bool {
	switch op {
	case OpReplaceSMTPAuth, OpReplaceStoreCaps, OpReplaceHideExtensions, OpReplaceAdmission:
		return true
	default:
		return false
	}
}

// RevisionStatus is the public revision + store generation view.
type RevisionStatus struct {
	BootstrapRevision Revision   `json:"bootstrapRevision"`
	RuntimeRevision   Revision   `json:"runtimeRevision"`
	Generation        Generation `json:"generation"`
	StoreGeneration   uint64     `json:"storeGeneration"`
	Drifted           bool       `json:"drifted"`
	MessageCount      int        `json:"messageCount"`
	StoreBytes        int64      `json:"storeBytes"`
	LoadedAt          string     `json:"loadedAt"`
}
