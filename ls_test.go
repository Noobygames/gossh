package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/noobygames/gossh/pkg/config"
)

func TestResolveRemotePath(t *testing.T) {
	tests := []struct {
		name      string
		pathArg   string
		remoteDir string
		cfgRemDir string
		want      string
	}{
		{"explicit path arg wins over all", "/explicit", "/flag", "/config", "/explicit"},
		{"flag remote-dir wins over config", "", "/flag", "/config", "/flag"},
		{"config remote-dir wins over default", "", "", "/config", "/config"},
		{"default ~/kubernetes when nothing set", "", "", "", "~/kubernetes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{RemoteDir: tt.cfgRemDir}
			assert.Equal(t, tt.want, resolveRemotePath(tt.pathArg, tt.remoteDir, cfg))
		})
	}
}
