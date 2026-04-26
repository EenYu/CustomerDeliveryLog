package mysql

import (
	"reflect"
	"testing"
)

func TestPlanUserUsernameIndexMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		indexes       []tableIndex
		wantDrop      []string
		wantComposite bool
	}{
		{
			name: "old single unique index",
			indexes: []tableIndex{
				{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
				{Name: "uk_username", Unique: true, Columns: []string{"username"}},
			},
			wantDrop:      []string{"uk_username"},
			wantComposite: true,
		},
		{
			name: "already migrated",
			indexes: []tableIndex{
				{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
				{Name: "uk_username_deleted", Unique: true, Columns: []string{"username", "is_deleted"}},
			},
			wantDrop:      nil,
			wantComposite: false,
		},
		{
			name: "both old and new indexes exist",
			indexes: []tableIndex{
				{Name: "PRIMARY", Unique: true, Columns: []string{"id"}},
				{Name: "uk_username", Unique: true, Columns: []string{"username"}},
				{Name: "uk_username_deleted", Unique: true, Columns: []string{"username", "is_deleted"}},
			},
			wantDrop:      []string{"uk_username"},
			wantComposite: false,
		},
		{
			name: "ignore unrelated indexes",
			indexes: []tableIndex{
				{Name: "idx_user_deleted", Unique: false, Columns: []string{"is_deleted"}},
				{Name: "idx_real_name", Unique: false, Columns: []string{"real_name"}},
			},
			wantDrop:      nil,
			wantComposite: false,
		},
		{
			name: "case insensitive columns",
			indexes: []tableIndex{
				{Name: "uk_username", Unique: true, Columns: []string{"USERNAME"}},
			},
			wantDrop:      []string{"uk_username"},
			wantComposite: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotDrop, gotComposite := planUserUsernameIndexMigration(tt.indexes)
			if !reflect.DeepEqual(normalizeStrings(gotDrop), normalizeStrings(tt.wantDrop)) {
				t.Fatalf("drop indexes mismatch: got %v, want %v", gotDrop, tt.wantDrop)
			}
			if gotComposite != tt.wantComposite {
				t.Fatalf("add composite mismatch: got %v, want %v", gotComposite, tt.wantComposite)
			}
		})
	}
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return values
}
