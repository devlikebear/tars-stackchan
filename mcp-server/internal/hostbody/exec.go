package hostbody

import (
	"context"
	"os/exec"
)

type Runner interface {
	Run(context.Context, string, ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command string, args ...string) error {
	return exec.CommandContext(ctx, command, args...).Run()
}
