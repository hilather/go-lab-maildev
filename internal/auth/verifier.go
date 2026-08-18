package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
	"github.com/hilather/go-lab-maildev/internal/model"
)

const (
	realmBearer = `Bearer realm="labmail"`
	realmBasic  = `Basic realm="labmail"`
)

// Verifier is the process-local token + Basic index.
type Verifier struct {
	mu     sync.RWMutex
	mode   string
	tokens []storedToken
	basic  *basicCred
}

type storedToken struct {
	id     string
	role   string
	scopes []string
	digest [sha256.Size]byte
}

type basicCred struct {
	username string
	password [sha256.Size]byte
	tokenIdx int
}

// Request is one authentication attempt. Adapters fill it from the HTTP
// request; X-Forwarded-For is never consulted.
type Request struct {
	Authorization string
	RemoteAddr    string
	// AllowBasic is true for REST and compat. MCP must leave it false.
	AllowBasic bool
}

// FromSpec compiles management.auth. Missing secret files fail closed.
func FromSpec(spec model.MgmtAuthSpec) (*Verifier, error) {
	mode := strings.TrimSpace(spec.Mode)
	if mode == "" {
		mode = model.MgmtAuthBearerAndBasic
	}
	switch mode {
	case model.MgmtAuthBearer, model.MgmtAuthBearerAndBasic, model.MgmtAuthDevLoopbackUnauth:
	default:
		return nil, domainerr.ValidationFailed("unknown auth mode",
			domainerr.FieldViolation{Path: "spec.management.auth.mode", Code: "invalid_value", Message: "unknown auth mode"})
	}

	seenID := map[string]int{}
	seenDigest := map[[sha256.Size]byte]string{}
	tokens := make([]storedToken, 0, len(spec.Tokens))
	for i, tok := range spec.Tokens {
		id := strings.TrimSpace(tok.ID)
		if id == "" {
			return nil, domainerr.ValidationFailed("token id is required",
				domainerr.FieldViolation{Path: indexPath("spec.management.auth.tokens", i) + ".id", Code: "empty_id", Message: "token id is required"})
		}
		if _, ok := seenID[id]; ok {
			return nil, domainerr.ValidationFailed("duplicate token id",
				domainerr.FieldViolation{Path: indexPath("spec.management.auth.tokens", i) + ".id", Code: "duplicate_id", Message: "duplicate token id"})
		}
		raw, err := readSecretFile(tok.SecretFile)
		if err != nil {
			return nil, domainerr.ValidationFailed("token secret is unavailable",
				domainerr.FieldViolation{Path: indexPath("spec.management.auth.tokens", i) + ".secretFile", Code: "unresolved_reference", Message: "token secret file does not resolve"})
		}
		if len(raw) < MinTokenBytes {
			zero(raw)
			return nil, domainerr.ValidationFailed("token entropy is below 256 bits",
				domainerr.FieldViolation{Path: indexPath("spec.management.auth.tokens", i) + ".secretFile", Code: "invalid_value", Message: "token secret must be at least 32 bytes"})
		}
		d := DigestSecret(raw)
		zero(raw)
		if other, ok := seenDigest[d]; ok {
			return nil, domainerr.ValidationFailed("duplicate token value",
				domainerr.FieldViolation{Path: indexPath("spec.management.auth.tokens", i) + ".secretFile", Code: "duplicate_id", Message: "token value matches " + other})
		}
		if tok.Role != "" && !model.KnownRole(tok.Role) {
			zero(raw)
			return nil, domainerr.ValidationFailed("unknown role",
				domainerr.FieldViolation{Path: indexPath("spec.management.auth.tokens", i) + ".role", Code: "invalid_value", Message: "role must be viewer, operator, or administrator"})
		}
		seenID[id] = len(tokens)
		seenDigest[d] = id
		role, scopes := expandScopes(tok.Role, tok.Scopes)
		tokens = append(tokens, storedToken{id: id, role: role, scopes: scopes, digest: d})
	}

	v := &Verifier{mode: mode, tokens: tokens}

	basic := spec.Basic
	if mode == model.MgmtAuthBearerAndBasic && strings.TrimSpace(basic.Username) != "" {
		ref := strings.TrimSpace(basic.TokenRef)
		idx, ok := seenID[ref]
		if !ok {
			return nil, domainerr.ValidationFailed("basic.tokenRef does not match a token id",
				domainerr.FieldViolation{Path: "spec.management.auth.basic.tokenRef", Code: "unresolved_reference", Message: "basic.tokenRef does not match a token id"})
		}
		pw, err := readSecretFile(basic.PasswordFile)
		if err != nil {
			return nil, domainerr.ValidationFailed("basic password is unavailable",
				domainerr.FieldViolation{Path: "spec.management.auth.basic.passwordFile", Code: "unresolved_reference", Message: "basic password file does not resolve"})
		}
		if len(pw) == 0 {
			return nil, domainerr.ValidationFailed("basic password is empty",
				domainerr.FieldViolation{Path: "spec.management.auth.basic.passwordFile", Code: "required", Message: "basic password is empty"})
		}
		v.basic = &basicCred{
			username: strings.TrimSpace(basic.Username),
			password: DigestSecret(pw),
			tokenIdx: idx,
		}
		zero(pw)
	}
	return v, nil
}

// Replace swaps the compiled index in place so REST/MCP/compat share one pointer.
func (v *Verifier) Replace(next *Verifier) {
	if v == nil || next == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.mode = next.mode
	v.tokens = next.tokens
	v.basic = next.basic
}

// Equivalent reports whether the compiled identity (mode, token digests, Basic) matches.
func (v *Verifier) Equivalent(other *Verifier) bool {
	if v == nil || other == nil {
		return v == other
	}
	modeA, toksA, basicA := v.snapshot()
	modeB, toksB, basicB := other.snapshot()
	if modeA != modeB || len(toksA) != len(toksB) {
		return false
	}
	byID := make(map[string][sha256.Size]byte, len(toksA))
	for _, t := range toksA {
		byID[t.id] = t.digest
	}
	for _, t := range toksB {
		d, ok := byID[t.id]
		if !ok || !EqualDigest(d, t.digest) {
			return false
		}
	}
	if (basicA == nil) != (basicB == nil) {
		return false
	}
	if basicA != nil && (basicA.username != basicB.username || basicA.tokenIdx != basicB.tokenIdx || !EqualDigest(basicA.password, basicB.password)) {
		return false
	}
	return true
}

func (v *Verifier) snapshot() (string, []storedToken, *basicCred) {
	if v == nil {
		return "", nil, nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	toks := append([]storedToken(nil), v.tokens...)
	var b *basicCred
	if v.basic != nil {
		cp := *v.basic
		b = &cp
	}
	return v.mode, toks, b
}

// Mode is the compiled auth mode.
func (v *Verifier) Mode() string {
	if v == nil {
		return ""
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.mode
}

// BasicEnabled reports whether HTTP Basic is an accepted REST/compat authenticator.
func (v *Verifier) BasicEnabled() bool {
	if v == nil {
		return false
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.mode == model.MgmtAuthBearerAndBasic && v.basic != nil
}

// WWWAuthenticate returns the 401 challenge list. MCP callers pass basic=false.
func WWWAuthenticate(basic bool) []string {
	if basic {
		return []string{realmBearer, realmBasic}
	}
	return []string{realmBearer}
}

// Authenticate verifies Authorization. A missing header is unauthenticated
// unless mode is dev-loopback-unauth and RemoteAddr is loopback.
func (v *Verifier) Authenticate(in Request) (Principal, error) {
	if v == nil {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	v.mu.RLock()
	defer v.mu.RUnlock()

	h := strings.TrimSpace(in.Authorization)
	if h == "" {
		if v.mode == model.MgmtAuthDevLoopbackUnauth && IsLoopback(in.RemoteAddr) {
			return loopbackPrincipal(), nil
		}
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}

	scheme, rest, ok := strings.Cut(h, " ")
	if !ok {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	rest = strings.TrimSpace(rest)
	switch {
	case strings.EqualFold(scheme, "Bearer"):
		return v.lookupBearerLocked(rest)
	case strings.EqualFold(scheme, "Basic"):
		if !in.AllowBasic || v.basic == nil || v.mode != model.MgmtAuthBearerAndBasic {
			return Principal{}, domainerr.Unauthenticated("authentication required")
		}
		return v.lookupBasicLocked(rest)
	default:
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
}

// AuthenticateBearer looks up a raw token secret (mcp-stdio --token-file).
func (v *Verifier) AuthenticateBearer(secret string) (Principal, error) {
	if v == nil {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.lookupBearerLocked(strings.TrimSpace(secret))
}

func (v *Verifier) lookupBearerLocked(secret string) (Principal, error) {
	if secret == "" || strings.ContainsAny(secret, " \t") {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	digest := DigestSecret([]byte(secret))
	found := 0
	idx := 0
	for i, t := range v.tokens {
		eq := 0
		if EqualDigest(t.digest, digest) {
			eq = 1
		}
		mask := eq
		idx = idx*(1-mask) + i*mask
		found += eq
	}
	if found != 1 {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	return principalOf(v.tokens[idx]), nil
}

func (v *Verifier) lookupBasicLocked(payload string) (Principal, error) {
	user, pass, ok := parseBasic(payload)
	// Always compare so a bad username does not skip the password digest.
	wantUser := ""
	wantPass := DigestSecret(nil)
	idx := 0
	if v.basic != nil {
		wantUser = v.basic.username
		wantPass = v.basic.password
		idx = v.basic.tokenIdx
	}
	// Hash usernames so compare cost does not leak length.
	userOK := EqualDigest(DigestSecret([]byte(user)), DigestSecret([]byte(wantUser)))
	passOK := EqualDigest(DigestSecret([]byte(pass)), wantPass)
	if !ok || !userOK || !passOK || idx < 0 || idx >= len(v.tokens) {
		return Principal{}, domainerr.Unauthenticated("authentication required")
	}
	return principalOf(v.tokens[idx]), nil
}

func principalOf(t storedToken) Principal {
	return Principal{
		ID:     t.id,
		Class:  ClassToken,
		Role:   t.role,
		Scopes: append([]string(nil), t.scopes...),
	}
}

func loopbackPrincipal() Principal {
	return Principal{
		ID:     "loopback",
		Class:  ClassLoopback,
		Role:   model.RoleAdministrator,
		Scopes: allScopes(),
	}
}

func parseBasic(payload string) (user, pass string, ok bool) {
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		// Some clients omit padding.
		raw, err = base64.StdEncoding.DecodeString(payload + strings.Repeat("=", (4-len(payload)%4)%4))
		if err != nil {
			return "", "", false
		}
	}
	s := string(raw)
	u, p, found := strings.Cut(s, ":")
	if !found {
		return "", "", false
	}
	return u, p, true
}

func readSecretFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return []byte(line), nil
	}
	return nil, os.ErrInvalid
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func indexPath(base string, i int) string {
	return strings.TrimSuffix(base, ".") + "[" + strconv.Itoa(i) + "]"
}

// IsLoopback reports whether remoteAddr is a loopback host (with or without port).
func IsLoopback(remoteAddr string) bool {
	host := strings.TrimSpace(remoteAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "localhost" {
		return true
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	return addr.IsLoopback()
}
