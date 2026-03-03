package run

import (
	"reflect"
	"testing"
)

func TestInheritedCadences(t *testing.T) {
	tests := []struct {
		name    string
		cadence string
		want    []string
	}{
		{name: "daily", cadence: "daily", want: []string{"daily"}},
		{name: "weekly", cadence: "weekly", want: []string{"daily", "weekly"}},
		{name: "monthly", cadence: "monthly", want: []string{"daily", "weekly", "monthly"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := inheritedCadences(tt.cadence)
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected cadence chain: got %v want %v", got, tt.want)
			}
		})
	}
}

func TestInheritedCadencesRejectsUnknownCadence(t *testing.T) {
	if _, err := inheritedCadences("yearly"); err == nil {
		t.Fatalf("expected error")
	}
}
