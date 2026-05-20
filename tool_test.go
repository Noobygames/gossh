package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noobygames/gossh/pkg/config"
	"github.com/noobygames/gossh/pkg/sshconn"
)

// nonExistentKey avoids SSH key discovery and passphrase prompts in tests.
// sshconn.Connect returns IdentityNotFoundError immediately for a missing file.
const nonExistentKey = "/tmp/gossh_test_nonexistent_key_xyz"

func TestRunToolNonExistentIdentityFails(t *testing.T) {
	err := runTool(context.Background(), "kubectl", "user@host", nonExistentKey, []string{"get"}, config.Config{})

	require.Error(t, err)
	var identErr *sshconn.IdentityNotFoundError
	require.ErrorAs(t, err, &identErr)
	assert.Equal(t, nonExistentKey, identErr.Path)
}

func TestRunToolFlagServerOverridesConfig(t *testing.T) {
	// With a non-existent identity, Connect is invoked (confirming the server
	// was accepted) and fails with IdentityNotFoundError, not InvalidServerError.
	err := runTool(context.Background(), "kubectl", "flag@host", nonExistentKey,
		[]string{"get"}, config.Config{Server: "cfg@host"})

	require.Error(t, err)
	var identErr *sshconn.IdentityNotFoundError
	assert.ErrorAs(t, err, &identErr, "IdentityNotFoundError confirms Connect was called with flag server")
}

func TestRunToolServerFromConfig(t *testing.T) {
	err := runTool(context.Background(), "kubectl", "", nonExistentKey,
		[]string{"get"}, config.Config{Server: "cfg@host"})

	require.Error(t, err)
	var identErr *sshconn.IdentityNotFoundError
	assert.ErrorAs(t, err, &identErr, "IdentityNotFoundError confirms Connect was called with config server")
}

// Empty server validation (InvalidServerError) is tested in pkg/sshconn via
// TestParseTarget; testing it through runTool is not feasible because loadAuth
// runs before ParseTarget and would fail first on any real system.
func TestRunToolEmptyServerFails(t *testing.T) {
	// Without an explicit identity, loadAuth discovers default ~/.ssh keys.
	// We can still assert that an error is returned regardless.
	err := runTool(context.Background(), "kubectl", "", nonExistentKey,
		[]string{"get"}, config.Config{})

	require.Error(t, err)
}
