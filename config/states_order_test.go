package config

import "testing"

// Regression for the ls_state_order/catalog split: a state declared only in
// ls_state_order (BACKLOG here) must be a known catalog state and hold its
// declared position, even when explicit states: are also present. Previously
// the ordering was discarded whenever states: existed, so doctor flagged
// thousands of order-declared states as unknown while ls grouped by them.
func TestOrderOnlyStatesRegisterInCatalog(t *testing.T) {
	cfg := &Config{
		States: []StateConfig{
			{Name: "TODO", Aliases: []string{"todo"}, TuiHotkey: "t"},
			{Name: "DONE", Aliases: []string{"done"}, TuiHotkey: "d"},
		},
		LsStateOrder: []string{"BACKLOG", "TODO", "DONE"},
	}
	catalog, err := BuildStateCatalog(cfg)
	if err != nil {
		t.Fatalf("BuildStateCatalog: %v", err)
	}
	if _, known := catalog.Resolve("BACKLOG"); !known {
		t.Fatal("expected order-only state BACKLOG to resolve in the catalog")
	}
	if _, known := catalog.Resolve("backlog"); !known {
		t.Fatal("expected order-only state to resolve case-insensitively")
	}
	ordered := catalog.SortStates()
	if len(ordered) == 0 || ordered[0].Name != "BACKLOG" {
		t.Fatalf("expected BACKLOG first per ls_state_order, got %v", ordered)
	}
	if _, known := catalog.Resolve("NEVERSEEN"); known {
		t.Fatal("expected truly undeclared state to stay unknown")
	}
}
