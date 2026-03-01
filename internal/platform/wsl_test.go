package platform

import "testing"

func TestValidatePassesInWSL(t *testing.T) {
	guard := NewWSLGuard(func(key string) string {
		if key == "WSL_DISTRO_NAME" {
			return "Fedora"
		}
		return ""
	})

	if err := guard.Validate(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidateFailsOutsideWSL(t *testing.T) {
	guard := NewWSLGuard(func(string) string { return "" })
	if err := guard.Validate(); err == nil {
		t.Fatalf("expected error")
	}
}
