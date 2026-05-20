package sshconn_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noobygames/gossh/pkg/sshconn"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantUser string
		wantHost string
		wantErr  bool
	}{
		{"host without port", "user@host", "user", "host:22", false},
		{"host with port", "user@host:2222", "user", "host:2222", false},
		{"ip address", "admin@192.168.1.1", "admin", "192.168.1.1:22", false},
		{"missing at sign", "noatsign", "", "", true},
		{"empty user", "@host", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, host, err := sshconn.ParseTarget(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				var invErr *sshconn.InvalidServerError
				require.ErrorAs(t, err, &invErr, "expected InvalidServerError")
				assert.Equal(t, tt.input, invErr.Server)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantUser, user)
			assert.Equal(t, tt.wantHost, host)
		})
	}
}
