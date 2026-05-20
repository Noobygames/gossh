package main

import (
	"context"

	"github.com/noobygames/gossh/pkg/config"
	"github.com/noobygames/gossh/pkg/remoteexec"
)

func cmdTool(ctx context.Context, tool string, args []string) error {
	server, identity, toolArgs := remoteexec.ExtractGosshFlags(args)

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return runTool(ctx, tool, server, identity, toolArgs, cfg)
}

func runTool(ctx context.Context, tool, server, identity string, toolArgs []string, cfg config.Config) error {
	return remoteexec.Exec(
		ctx,
		config.FirstNonEmpty(server, cfg.Server),
		config.FirstNonEmpty(identity),
		tool,
		toolArgs,
	)
}
