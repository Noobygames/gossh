package main

import "testing"

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input    string
		wantUser string
		wantHost string
		wantErr  bool
	}{
		{"user@host", "user", "host:22", false},
		{"user@host:2222", "user", "host:2222", false},
		{"admin@192.168.1.1", "admin", "192.168.1.1:22", false},
		{"noatsign", "", "", true},
		{"@host", "", "", true},
	}
	for _, tt := range tests {
		user, host, err := parseTarget(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseTarget(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if user != tt.wantUser {
			t.Errorf("parseTarget(%q) user = %q, want %q", tt.input, user, tt.wantUser)
		}
		if host != tt.wantHost {
			t.Errorf("parseTarget(%q) host = %q, want %q", tt.input, host, tt.wantHost)
		}
	}
}
