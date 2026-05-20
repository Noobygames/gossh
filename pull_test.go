package main

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/noobygames/gossh/pkg/config"
)

func TestRunPullTooManyArgs(t *testing.T) {
	err := runPull(context.Background(), "", "", "",
		[]string{"a", "b", "c"}, config.Config{}, io.Discard)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many arguments")
}
