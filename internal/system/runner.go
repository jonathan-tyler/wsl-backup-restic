package system

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Executor interface {
	Run(ctx context.Context, name string, args ...string) error
	RunWithEnv(ctx context.Context, env map[string]string, name string, args ...string) error
	RunCapture(ctx context.Context, name string, args ...string) (string, error)
}

type OSExecutor struct {
	stdout io.Writer
	stderr io.Writer
}

func NewOSExecutor(stdout io.Writer, stderr io.Writer) OSExecutor {
	return OSExecutor{stdout: stdout, stderr: stderr}
}

var commandContext = exec.CommandContext

func (e OSExecutor) Run(ctx context.Context, name string, args ...string) error {
	return e.RunWithEnv(ctx, nil, name, args...)
}

func (e OSExecutor) RunWithEnv(ctx context.Context, env map[string]string, name string, args ...string) error {
	fmt.Fprintf(e.stdout, "\n$ %s\n", formatLine(name, args))

	cmd := commandContext(ctx, name, args...)
	if len(env) > 0 {
		base := os.Environ()
		cmd.Env = mergeEnv(base, env)
	}
	cmd.Stdout = dimWriter(e.stdout)
	cmd.Stderr = dimWriter(e.stderr)

	return cmd.Run()
}

func (e OSExecutor) RunCapture(ctx context.Context, name string, args ...string) (string, error) {
	fmt.Fprintf(e.stdout, "\n$ %s\n", formatLine(name, args))

	cmd := commandContext(ctx, name, args...)
	var buffer bytes.Buffer
	cmd.Stdout = io.MultiWriter(dimWriter(e.stdout), &buffer)
	cmd.Stderr = io.MultiWriter(dimWriter(e.stderr), &buffer)

	err := cmd.Run()
	return buffer.String(), err
}

func mergeEnv(base []string, overrides map[string]string) []string {
	result := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		eq := strings.IndexByte(item, '=')
		if eq <= 0 {
			result = append(result, item)
			continue
		}
		key := item[:eq]
		if _, overridden := overrides[key]; overridden {
			continue
		}
		result = append(result, item)
	}

	for key, value := range overrides {
		result = append(result, key+"="+value)
	}

	return result
}
