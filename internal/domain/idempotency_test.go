package domain

import "testing"

func TestIdempotencyStatusValid(t *testing.T) {
	tests := []struct {
		name   string
		status IdempotencyStatus
		want   bool
	}{
		{name: "processing", status: IdempotencyStatusProcessing, want: true},
		{name: "done", status: IdempotencyStatusDone, want: true},
		{name: "failed", status: IdempotencyStatusFailed, want: true},
		{name: "invalid", status: IdempotencyStatus("broken"), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.Valid(); got != tc.want {
				t.Fatalf("status %q valid=%v, want %v", tc.status, got, tc.want)
			}
		})
	}
}
