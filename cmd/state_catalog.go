package cmd

import (
	"fmt"
	"punchlist/config"
	"punchlist/task"
	"sort"
	"strings"
)

func loadStateCatalog() (*config.StateCatalog, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return config.DefaultStateCatalog(), nil
	}
	catalog, err := config.BuildStateCatalog(cfg)
	if err != nil {
		return nil, err
	}
	return catalog, nil
}

func canonicalizeState(catalog *config.StateCatalog, state task.State) string {
	if catalog == nil {
		return strings.ToUpper(strings.TrimSpace(string(state)))
	}
	return catalog.Canonicalize(string(state))
}

func compareState(catalog *config.StateCatalog, state task.State, filter string) bool {
	if filter == "" {
		return true
	}
	if catalog == nil {
		return strings.EqualFold(strings.TrimSpace(string(state)), strings.TrimSpace(filter))
	}
	stateCanon := catalog.Canonicalize(string(state))
	filterCanon := catalog.Canonicalize(filter)
	return stateCanon == filterCanon
}

func buildStateOrderIndex(catalog *config.StateCatalog, tasks []*task.Task) map[string]int {
	orderIndex := map[string]int{}
	if catalog != nil {
		for i, name := range catalog.Order {
			key := strings.ToUpper(strings.TrimSpace(name))
			if key != "" {
				orderIndex[key] = i
			}
		}
	}

	unknownStates := map[string]struct{}{}
	for _, t := range tasks {
		canonical := canonicalizeState(catalog, t.State)
		key := strings.ToUpper(strings.TrimSpace(canonical))
		if key == "" {
			continue
		}
		if _, ok := orderIndex[key]; ok {
			continue
		}
		unknownStates[key] = struct{}{}
	}

	if len(unknownStates) == 0 {
		return orderIndex
	}

	extraKeys := make([]string, 0, len(unknownStates))
	for key := range unknownStates {
		extraKeys = append(extraKeys, key)
	}
	sort.Strings(extraKeys)
	nextIndex := len(orderIndex)
	for _, key := range extraKeys {
		if _, ok := orderIndex[key]; ok {
			continue
		}
		orderIndex[key] = nextIndex
		nextIndex++
	}
	return orderIndex
}

func orderIndexForState(orderIndex map[string]int, state string) (int, error) {
	key := strings.ToUpper(strings.TrimSpace(state))
	if key == "" {
		return 0, fmt.Errorf("empty state")
	}
	if idx, ok := orderIndex[key]; ok {
		return idx, nil
	}
	return len(orderIndex), nil
}
