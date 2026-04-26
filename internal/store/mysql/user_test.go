package mysql

import (
	"strconv"
	"strings"
	"testing"
)

func TestDeletedUsername(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		username string
		userID   int64
	}{
		{
			name:     "short username",
			username: "public",
			userID:   101,
		},
		{
			name:     "long username is truncated",
			username: "abcdefghijklmnopqrstuvwxyz1234567890longusername",
			userID:   202,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := deletedUsername(tt.username, tt.userID)
			if len(got) > 50 {
				t.Fatalf("expected deleted username length <= 50, got %d (%q)", len(got), got)
			}
			if !strings.HasSuffix(got, "~del~"+strconv.FormatInt(tt.userID, 10)) {
				t.Fatalf("expected deleted username to end with user id suffix, got %q", got)
			}
			if got == tt.username {
				t.Fatalf("expected deleted username to change, got %q", got)
			}
		})
	}
}
