package auth

import (
	"net"
	"net/url"
	"strings"

	"github.com/hilather/go-lab-maildev/internal/domainerr"
)

const (
	originAllowAny     = "*"
	originAllowPrivate = "private"
)

// CheckOrigin implements the LabDNS wording: a present non-loopback Origin
// is rejected unless it is on originAllowlist. Missing Origin is allowed.
// Only http/https Origins are accepted (file:// is denied even on loopback).
// Allowlist sentinels: "*" (any remaining http(s) Origin) and "private"
// (Go net.IP.IsPrivate host — RFC 1918 and RFC 4193 ULA, not CGNAT).
func CheckOrigin(origin string, allowlist []string) error {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return domainerr.Forbidden("origin is not allowed")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return domainerr.Forbidden("origin is not allowed")
	}
	host := u.Hostname()
	if isLoopbackHost(host) {
		return nil
	}
	for _, allowed := range allowlist {
		raw := strings.TrimSpace(allowed)
		switch {
		case raw == originAllowAny:
			return nil
		case strings.EqualFold(raw, originAllowPrivate) && isPrivateOriginHost(host):
			return nil
		case originMatches(origin, allowed):
			return nil
		}
	}
	return domainerr.Forbidden("origin is not allowed")
}

func isPrivateOriginHost(host string) bool {
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsPrivate()
}

func originMatches(got, want string) bool {
	got = strings.TrimRight(strings.TrimSpace(got), "/")
	want = strings.TrimRight(strings.TrimSpace(want), "/")
	return strings.EqualFold(got, want)
}

func isLoopbackHost(host string) bool {
	h := strings.TrimSpace(host)
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}
