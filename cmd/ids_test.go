package cmd

import (
	"reflect"
	"testing"
)

// test parsing of id selectors
func TestParseTaskIDs(t *testing.T) {
	t.Run("single ids and ranges", func(t *testing.T) {
		ids, err := parseTaskIDs([]string{"2", "3", "5-7"})
		if err != nil {
			t.Fatalf("parseTaskIDs failed: %v", err)
		}
		expected := []int{2, 3, 5, 6, 7}
		if !reflect.DeepEqual(ids, expected) {
			t.Fatalf("expected %v, got %v", expected, ids)
		}
	})

	t.Run("bracket selector single arg", func(t *testing.T) {
		ids, err := parseTaskIDs([]string{"[1,2,4-5]"})
		if err != nil {
			t.Fatalf("parseTaskIDs failed: %v", err)
		}
		expected := []int{1, 2, 4, 5}
		if !reflect.DeepEqual(ids, expected) {
			t.Fatalf("expected %v, got %v", expected, ids)
		}
	})

	t.Run("bracket selector split args", func(t *testing.T) {
		ids, err := parseTaskIDs([]string{"[1,", "2,", "4-5]"})
		if err != nil {
			t.Fatalf("parseTaskIDs failed: %v", err)
		}
		expected := []int{1, 2, 4, 5}
		if !reflect.DeepEqual(ids, expected) {
			t.Fatalf("expected %v, got %v", expected, ids)
		}
	})

	t.Run("bracket selector must be only argument", func(t *testing.T) {
		_, err := parseTaskIDs([]string{"[1,2]", "3"})
		if err == nil {
			t.Fatal("expected error for extra args after bracket selector")
		}
	})

	t.Run("invalid tokens", func(t *testing.T) {
		cases := [][]string{
			{"[1-]"},
			{"[a]"},
			{"a"},
		}
		for _, args := range cases {
			if _, err := parseTaskIDs(args); err == nil {
				t.Fatalf("expected error for args %v", args)
			}
		}
	})
}
