package auth

import "testing"

func TestCheckOriginTable(t *testing.T) {
	t.Parallel()
	allow := []string{"https://ui.lab.test"}
	cases := []struct {
		name, origin string
		extra        []string
		ok           bool
	}{
		{name: "missing", origin: "", ok: true},
		{name: "loopback-http", origin: "http://127.0.0.1:1080", ok: true},
		{name: "loopback-localhost", origin: "http://localhost:1080", ok: true},
		{name: "evil", origin: "https://evil.example", ok: false},
		{name: "file-loopback", origin: "file://localhost", ok: false},
		{name: "allowlist-hit", origin: "https://ui.lab.test", extra: allow, ok: true},
		{name: "allowlist-miss", origin: "https://other.example", extra: allow, ok: false},
		{name: "star-allows-evil", origin: "https://evil.example", extra: []string{"*"}, ok: true},
		{name: "star-allows-lan", origin: "http://192.168.1.9:1080", extra: []string{"*"}, ok: true},
		{name: "star-denies-file", origin: "file://localhost", extra: []string{"*"}, ok: false},
		{name: "star-denies-null", origin: "null", extra: []string{"*"}, ok: false},
		{name: "star-denies-star-slash", origin: "https://evil.example", extra: []string{"*/"}, ok: false},
		{name: "star-coexists", origin: "https://evil.example", extra: []string{"https://ui.lab.test", "*"}, ok: true},
		{name: "private-lan", origin: "http://192.168.1.9:1080", extra: []string{"private"}, ok: true},
		{name: "private-10", origin: "http://10.1.2.3:1080", extra: []string{"private"}, ok: true},
		{name: "private-ula", origin: "http://[fd12:3456::1]:1080", extra: []string{"private"}, ok: true},
		{name: "private-cgnat", origin: "http://100.64.0.1:1080", extra: []string{"private"}, ok: false},
		{name: "private-mapped-v4", origin: "http://[::ffff:192.168.1.9]:1080", extra: []string{"private"}, ok: true},
		{name: "private-denies-evil", origin: "https://evil.example", extra: []string{"private"}, ok: false},
		{name: "private-denies-hostname", origin: "http://devbox:1080", extra: []string{"private"}, ok: false},
		{name: "private-denies-link-local-v4", origin: "http://169.254.1.1:1080", extra: []string{"private"}, ok: false},
		{name: "private-denies-link-local-v6", origin: "http://[fe80::1]:1080", extra: []string{"private"}, ok: false},
		{name: "loopback-still-ok-empty", origin: "http://127.0.0.1:1080", extra: []string{}, ok: true},
		{name: "localhost-exact-case", origin: "http://LocalHost:1080", extra: []string{}, ok: false},
		{name: "exact-still-required", origin: "http://devbox:1080", extra: []string{}, ok: false},
		{name: "exact-hit", origin: "http://devbox:1080", extra: []string{"http://devbox:1080"}, ok: true},
		{name: "case-sentinel", origin: "http://10.0.0.1:1", extra: []string{"PRIVATE"}, ok: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckOrigin(tc.origin, tc.extra)
			if tc.ok && err != nil {
				t.Fatalf("denied: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("allowed")
			}
		})
	}
}
