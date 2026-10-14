package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestExitCodeOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"findings", &exitError{code: exitFindings}, exitFindings},
		{"wrapped", fmt.Errorf("ctx: %w", &exitError{code: exitFindings}), exitFindings},
		{"plain error", errors.New("boom"), exitFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCodeOf(tt.err); got != tt.want {
				t.Fatalf("exitCodeOf = %d, want %d", got, tt.want)
			}
		})
	}
}
