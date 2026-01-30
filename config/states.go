package config

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// StateConfig defines a canonical state and its aliases/hotkey.
type StateConfig struct {
	Name      string   `yaml:"name"`
	Aliases   []string `yaml:"aliases,omitempty"`
	TuiHotkey string   `yaml:"tui_hotkey,omitempty"`
}

// StateCatalog resolves state tokens, ordering, and hotkeys.
type StateCatalog struct {
	States        []StateConfig
	Order         []string
	tokenToState  map[string]string
	hotkeyToState map[string]string
}

// DefaultStates returns the default state definitions for new configs.
func DefaultStates() []StateConfig {
	return []StateConfig{
		{
			Name:      "TODO",
			Aliases:   []string{"todo"},
			TuiHotkey: "t",
		},
		{
			Name:      "BEGUN",
			Aliases:   []string{"begun", "STARTED", "started", "INPROGRESS", "inprogress"},
			TuiHotkey: "b",
		},
		{
			Name:      "FOLLOWUP",
			Aliases:   []string{"followup", "CONFIRM", "confirm"},
			TuiHotkey: "f",
		},
		{
			Name:      "DEFER",
			Aliases:   []string{"defer"},
			TuiHotkey: "l",
		},
		{
			Name:      "NOTDO",
			Aliases:   []string{"notdo"},
			TuiHotkey: "x",
		},
		{
			Name:      "DONE",
			Aliases:   []string{"done", "COMPLETE", "complete", "COMPLETED", "completed"},
			TuiHotkey: "d",
		},
	}
}

// DefaultStateCatalog returns a catalog based on default states.
func DefaultStateCatalog() *StateCatalog {
	catalog, err := BuildStateCatalog(&Config{
		States: DefaultStates(),
	})
	if err != nil {
		return &StateCatalog{
			States: DefaultStates(),
			Order:  DefaultLsStateOrder(),
		}
	}
	return catalog
}

// BuildStateCatalog builds a catalog from config, validating tokens and hotkeys.
func BuildStateCatalog(cfg *Config) (*StateCatalog, error) {
	states, order := effectiveStatesAndOrder(cfg)

	stateByToken := make(map[string]string, len(states))
	stateNames := make(map[string]struct{}, len(states))
	for _, st := range states {
		name := strings.TrimSpace(st.Name)
		if err := validateStateToken(name); err != nil {
			return nil, fmt.Errorf("invalid state name %q: %w", st.Name, err)
		}
		nameKey := strings.ToUpper(name)
		if _, ok := stateNames[nameKey]; ok {
			return nil, fmt.Errorf("duplicate state name %q", name)
		}
		stateNames[nameKey] = struct{}{}
		stateByToken[nameKey] = name

		for _, alias := range st.Aliases {
			alias = strings.TrimSpace(alias)
			if err := validateStateToken(alias); err != nil {
				return nil, fmt.Errorf("invalid alias %q for state %q: %w", alias, name, err)
			}
			aliasKey := strings.ToUpper(alias)
			if existing, ok := stateByToken[aliasKey]; ok {
				if existing == name {
					continue
				}
				return nil, fmt.Errorf("duplicate alias/token %q", alias)
			}
			stateByToken[aliasKey] = name
		}
	}

	normalizedOrder := normalizeOrder(order, stateByToken)
	for _, name := range normalizedOrder {
		nameKey := strings.ToUpper(name)
		if _, ok := stateNames[nameKey]; ok {
			continue
		}
		states = append(states, StateConfig{Name: name})
		stateNames[nameKey] = struct{}{}
		stateByToken[nameKey] = name
	}
	// ensure every defined state appears in ordering
	for _, st := range states {
		nameKey := strings.ToUpper(st.Name)
		if !containsToken(normalizedOrder, nameKey) {
			normalizedOrder = append(normalizedOrder, st.Name)
		}
	}

	hotkeyMap := make(map[string]string)
	for _, st := range states {
		if strings.TrimSpace(st.TuiHotkey) == "" {
			continue
		}
		key := strings.TrimSpace(st.TuiHotkey)
		if err := validateHotkey(key); err != nil {
			return nil, fmt.Errorf("invalid tui_hotkey %q for state %q: %w", key, st.Name, err)
		}
		if _, ok := hotkeyMap[key]; ok {
			return nil, fmt.Errorf("duplicate tui_hotkey %q", key)
		}
		hotkeyMap[key] = st.Name
	}

	return &StateCatalog{
		States:        states,
		Order:         normalizedOrder,
		tokenToState:  stateByToken,
		hotkeyToState: hotkeyMap,
	}, nil
}

// Resolve returns the canonical state name and whether it was known in the catalog.
func (c *StateCatalog) Resolve(token string) (string, bool) {
	if c == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", false
	}
	key := strings.ToUpper(trimmed)
	if name, ok := c.tokenToState[key]; ok {
		return name, true
	}
	return "", false
}

// Canonicalize returns the canonical state name if known, otherwise an uppercased token.
func (c *StateCatalog) Canonicalize(token string) string {
	if token == "" {
		return ""
	}
	if name, ok := c.Resolve(token); ok {
		return name
	}
	return strings.ToUpper(strings.TrimSpace(token))
}

// HotkeyMap returns a copy of the hotkey-to-state map.
func (c *StateCatalog) HotkeyMap() map[string]string {
	if c == nil {
		return nil
	}
	copyMap := make(map[string]string, len(c.hotkeyToState))
	for key, value := range c.hotkeyToState {
		copyMap[key] = value
	}
	return copyMap
}

func effectiveStatesAndOrder(cfg *Config) ([]StateConfig, []string) {
	if cfg == nil {
		return DefaultStates(), DefaultLsStateOrder()
	}
	if len(cfg.States) > 0 {
		order := make([]string, 0, len(cfg.States))
		for _, st := range cfg.States {
			order = append(order, st.Name)
		}
		return cfg.States, order
	}
	states := DefaultStates()
	if len(cfg.LsStateOrder) == 0 {
		return states, DefaultLsStateOrder()
	}
	return states, cfg.LsStateOrder
}

func normalizeOrder(order []string, tokenMap map[string]string) []string {
	normalized := make([]string, 0, len(order))
	seen := map[string]struct{}{}
	for _, label := range order {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key := strings.ToUpper(label)
		if canonical, ok := tokenMap[key]; ok {
			label = canonical
			key = strings.ToUpper(label)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, label)
	}
	return normalized
}

func containsToken(values []string, token string) bool {
	for _, value := range values {
		if strings.ToUpper(strings.TrimSpace(value)) == token {
			return true
		}
	}
	return false
}

func validateStateToken(token string) error {
	if token == "" {
		return fmt.Errorf("empty token")
	}
	for _, r := range token {
		if unicode.IsSpace(r) {
			return fmt.Errorf("state tokens must be single words")
		}
	}
	return nil
}

var reservedHotkeys = func() map[string]struct{} {
	keys := []string{
		"n", "q", "j", "k", "J", "K", " ", "e",
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
	}
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
}()

func validateHotkey(key string) error {
	if utf8.RuneCountInString(key) != 1 {
		return fmt.Errorf("hotkey must be a single character")
	}
	if key != strings.ToLower(key) {
		return fmt.Errorf("hotkey must be lowercase")
	}
	if _, ok := reservedHotkeys[key]; ok {
		return fmt.Errorf("hotkey %q is reserved", key)
	}
	if unicode.IsSpace([]rune(key)[0]) {
		return fmt.Errorf("hotkey cannot be whitespace")
	}
	return nil
}

// SortStates returns the catalog's states ordered by config order, falling back to name sort.
func (c *StateCatalog) SortStates() []StateConfig {
	if c == nil {
		return nil
	}
	orderIndex := make(map[string]int, len(c.Order))
	for i, name := range c.Order {
		orderIndex[strings.ToUpper(name)] = i
	}
	ordered := make([]StateConfig, len(c.States))
	copy(ordered, c.States)
	sort.SliceStable(ordered, func(i, j int) bool {
		ai := orderIndex[strings.ToUpper(ordered[i].Name)]
		aj := orderIndex[strings.ToUpper(ordered[j].Name)]
		if ai == aj {
			return strings.ToUpper(ordered[i].Name) < strings.ToUpper(ordered[j].Name)
		}
		return ai < aj
	})
	return ordered
}
