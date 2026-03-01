package prompt

import (
	"strings"
	"testing"
)

func TestNewYesNoConfirmAcceptsYes(t *testing.T) {
	input := strings.NewReader("yes\n")
	var output strings.Builder
	confirm := NewYesNoConfirm(input, &output)

	ok, err := confirm("Install now?")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !ok {
		t.Fatalf("expected confirmation to be true")
	}
	if !strings.Contains(output.String(), "Install now?") {
		t.Fatalf("expected prompt text in output")
	}
}

func TestNewYesNoConfirmDefaultsToNo(t *testing.T) {
	input := strings.NewReader("\n")
	var output strings.Builder
	confirm := NewYesNoConfirm(input, &output)

	ok, err := confirm("Install now?")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ok {
		t.Fatalf("expected confirmation to be false")
	}
}
