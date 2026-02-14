package domain

import (
	"errors"
	"testing"
)

func TestIsVersionConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "version conflict error",
			err:  ErrOrderVersionConflict,
			want: true,
		},
		{
			name: "wrapped version conflict error",
			err:  errors.Join(ErrOrderVersionConflict, errors.New("additional context")),
			want: true,
		},
		{
			name: "other error",
			err:  ErrOrderNotFound,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsVersionConflict(tt.err)
			if got != tt.want {
				t.Errorf("IsVersionConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIdempotencyConflict(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "idempotency already exists",
			err:  ErrIdempotencyKeyAlreadyExists,
			want: true,
		},
		{
			name: "idempotency hash mismatch",
			err:  ErrIdempotencyHashMismatch,
			want: true,
		},
		{
			name: "wrapped idempotency conflict",
			err:  errors.Join(ErrIdempotencyHashMismatch, errors.New("extra context")),
			want: true,
		},
		{
			name: "non idempotency error",
			err:  ErrOrderVersionConflict,
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsIdempotencyConflict(tt.err)
			if got != tt.want {
				t.Errorf("IsIdempotencyConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}
