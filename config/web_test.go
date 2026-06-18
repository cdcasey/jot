package config

import "testing"

func TestWebEnabledAndBindAddr(t *testing.T) {
	tests := []struct {
		name        string
		addr, port  string
		wantEnabled bool
		wantBind    string // only checked when wantEnabled
	}{
		{"both unset -> off", "", "", false, ""},
		{"port only -> loopback", "", "8080", true, "127.0.0.1:8080"},
		{"addr only -> default port", "100.64.0.1", "", true, "100.64.0.1:8080"},
		{"both set", "100.64.0.1", "9090", true, "100.64.0.1:9090"},
		{"ipv6 addr is bracketed", "::1", "8080", true, "[::1]:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{WebAddr: tt.addr, WebPort: tt.port}
			if got := c.WebEnabled(); got != tt.wantEnabled {
				t.Fatalf("WebEnabled() = %v, want %v", got, tt.wantEnabled)
			}
			if tt.wantEnabled {
				if got := c.WebBindAddr(); got != tt.wantBind {
					t.Errorf("WebBindAddr() = %q, want %q", got, tt.wantBind)
				}
			}
		})
	}
}
