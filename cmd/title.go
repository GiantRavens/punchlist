package cmd

import "punchlist/config"

func truncateWithEllipsis(input string, maxLen int) string {
	if maxLen <= 0 {
		return input
	}
	runes := []rune(input)
	if len(runes) <= maxLen {
		return input
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func titleMaxLenFromConfig(cfg *config.Config) int {
	if cfg == nil || cfg.TitleMaxLen <= 0 {
		return config.DefaultTitleMaxLen()
	}
	return cfg.TitleMaxLen
}

func loadLsTitleMaxLen() int {
	cfg, err := config.LoadConfig()
	if err != nil || cfg.LsTitleMaxLen <= 0 {
		return config.DefaultLsTitleMaxLen()
	}
	return cfg.LsTitleMaxLen
}
