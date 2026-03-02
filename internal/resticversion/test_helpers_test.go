package resticversion

import (
	"context"
	"strings"
)

type fakeSystemExecutor struct {
	captureOutput map[string]string
	captureErr    map[string]error
	runErr        map[string]error

	runCalls []string
}

func (e *fakeSystemExecutor) Run(_ context.Context, name string, args ...string) error {
	key := buildCmdKey(name, args...)
	e.runCalls = append(e.runCalls, key)
	if err, ok := e.runErr[key]; ok {
		return err
	}
	return nil
}

func (e *fakeSystemExecutor) RunWithEnv(ctx context.Context, _ map[string]string, name string, args ...string) error {
	return e.Run(ctx, name, args...)
}

func (e *fakeSystemExecutor) RunCapture(_ context.Context, name string, args ...string) (string, error) {
	key := buildCmdKey(name, args...)
	if out, ok := e.captureOutput[key]; ok {
		if err, hasErr := e.captureErr[key]; hasErr {
			return out, err
		}
		return out, nil
	}
	if err, ok := e.captureErr[key]; ok {
		return "", err
	}
	return "", nil
}

func buildCmdKey(name string, args ...string) string {
	return name + " " + strings.Join(args, " ")
}
