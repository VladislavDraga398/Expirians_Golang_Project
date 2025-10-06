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
