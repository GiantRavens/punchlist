package cmd

import (
	"reflect"
	"testing"
)

func TestRemoveTags(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		remove   []string
		want     []string
	}{
		{
			name:     "removes one, preserves order of the rest",
			existing: []string{"wip", "launch", "blocked"},
			remove:   []string{"launch"},
			want:     []string{"wip", "blocked"},
		},
		{
			name:     "removing an absent tag is a no-op",
			existing: []string{"wip", "launch"},
			remove:   []string{"gone"},
			want:     []string{"wip", "launch"},
		},
		{
			name:     "removes several at once",
			existing: []string{"a", "b", "c", "d"},
			remove:   []string{"b", "d"},
			want:     []string{"a", "c"},
		},
		{
			name:     "matches on trimmed value",
			existing: []string{" wip ", "launch"},
			remove:   []string{"wip"},
			want:     []string{"launch"},
		},
		{
			name:     "removing all leaves an empty slice",
			existing: []string{"only"},
			remove:   []string{"only"},
			want:     []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := removeTags(tc.existing, tc.remove)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("removeTags(%v, %v) = %v, want %v", tc.existing, tc.remove, got, tc.want)
			}
		})
	}
}

// `tag set` replaces the whole set; dedupeTags gives it its replace semantics,
// so a set with duplicates collapses to a deduped, order-preserving list.
func TestSetTagsDedupes(t *testing.T) {
	got := dedupeTags([]string{"wip", "launch", "wip", "blocked", "launch"})
	want := []string{"wip", "launch", "blocked"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("dedupeTags = %v, want %v", got, want)
	}
}
