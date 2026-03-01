package run

import (
	"context"
	"testing"

	"github.com/example/wsl-backup/internal/apperr"
)

type fakeRunner struct {
	args []string
	err  error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) error {
	f.args = append([]string{}, args...)
	return f.err
}

func TestHandleRequiresCadence(t *testing.T) {
	err := Handle(context.Background(), nil, &fakeRunner{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(apperr.UsageError); !ok {
		t.Fatalf("expected usage error, got %T", err)
	}
}

func TestHandleRejectsUnknownCadence(t *testing.T) {
	err := Handle(context.Background(), []string{"yearly"}, &fakeRunner{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(apperr.UsageError); !ok {
		t.Fatalf("expected usage error, got %T", err)
	}
}

func TestHandleBuildsResticArgs(t *testing.T) {
	runner := &fakeRunner{}
	err := Handle(context.Background(), []string{"weekly", "/data/source"}, runner)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	want := []string{"backup", "--tag", "cadence=weekly", "/data/source"}
	if len(runner.args) != len(want) {
		t.Fatalf("expected %d args, got %d (%#v)", len(want), len(runner.args), runner.args)
	}
	for i := range want {
		if runner.args[i] != want[i] {
			t.Fatalf("arg[%d]: expected %q, got %q", i, want[i], runner.args[i])
		}
	}
}
