package system

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

type Executor interface {
	Run(ctx context.Context, name string, args ...string) error
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
	fmt.Fprintf(e.stdout, "$ %s\n", formatLine(name, args))

	cmd := commandContext(ctx, name, args...)
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr

	return cmd.Run()
}

func (e OSExecutor) RunCapture(ctx context.Context, name string, args ...string) (string, error) {
	fmt.Fprintf(e.stdout, "$ %s\n", formatLine(name, args))

	cmd := commandContext(ctx, name, args...)
	var buffer bytes.Buffer
	cmd.Stdout = io.MultiWriter(e.stdout, &buffer)
	cmd.Stderr = io.MultiWriter(e.stderr, &buffer)

	err := cmd.Run()
	return buffer.String(), err
}

func formatCommand(args []string) string {
	if len(args) == 0 {
		return ""
	}

	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"") {
			quoted = append(quoted, strconv.Quote(arg))
			continue
		}
		quoted = append(quoted, arg)
	}

	return strings.Join(quoted, " ")
}

func formatLine(name string, args []string) string {
	command := formatCommand(args)
	if command == "" {
		return name
	}
	return name + " " + command
}
