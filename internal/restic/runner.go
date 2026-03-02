package restic

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Executor interface {
	Run(ctx context.Context, args ...string) error
}

type OSRunner struct {
	stdout io.Writer
	stderr io.Writer
}

func NewOSRunner(stdout io.Writer, stderr io.Writer) OSRunner {
	return OSRunner{stdout: stdout, stderr: stderr}
}

var commandContext = exec.CommandContext

func (r OSRunner) Run(ctx context.Context, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("restic command requires at least one argument")
	}

	password, err := loadResticPassword(ctx, r.stdout, r.stderr)
	if err != nil {
		return err
	}

	fmt.Fprintf(r.stdout, "\n$ restic %s\n", formatCommand(args))

	cmd := commandContext(ctx, "restic", args...)
	baseEnv := cmd.Env
	if len(baseEnv) == 0 {
		baseEnv = os.Environ()
	}
	cmd.Env = withResticPassword(baseEnv, password)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	return cmd.Run()
}

func withResticPassword(base []string, password string) []string {
	result := make([]string, 0, len(base)+1)
	prefix := "RESTIC_PASSWORD="
	for _, item := range base {
		if strings.HasPrefix(item, prefix) {
			continue
		}
		result = append(result, item)
	}

	return append(result, prefix+password)
}

func formatCommand(args []string) string {
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
