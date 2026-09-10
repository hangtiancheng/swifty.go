package mysql

import (
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm"
)

// TestIsDuplicateEntryErr verifies the duplicate-entry error detection.
func TestIsDuplicateEntryErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("some error"),
			want: false,
		},
		{
			name: "translated duplicate entry error",
			err:  gorm.ErrDuplicatedKey,
			want: true,
		},
		{
			name: "wrapped duplicate entry error",
			err:  fmt.Errorf("upsert failed: %w", gorm.ErrDuplicatedKey),
			want: true,
		},
		{
			name: "other gorm error",
			err:  gorm.ErrRecordNotFound,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDuplicateEntryErr(tt.err); got != tt.want {
				t.Errorf("IsDuplicateEntryErr(%v) = %t, want %t", tt.err, got, tt.want)
			}
		})
	}
}
