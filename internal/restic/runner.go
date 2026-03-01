package restic

import (
	"context"
	"fmt"
	"io"
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

	fmt.Fprintf(r.stdout, "$ restic %s\n", formatCommand(args))

	cmd := commandContext(ctx, "restic", args...)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	return cmd.Run()
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
