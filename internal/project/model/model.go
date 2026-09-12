// Package model holds built-in model presets — connection defaults for the
// models providers advertise, so a config.yaml entry can omit base_url,
// context_window, and image_enabled and still get the right values. Presets
// are keyed by the exact model name the API documents; unknown or legacy
// names fall through to the generic OpenAI defaults the config loader applies.
package model

import "github.com/pulseaiclub/phi/internal/llm"

// Lookup returns the built-in connection defaults for a model name. ok is
// false for names without a preset, so callers fall back to the generic
// OpenAI endpoint. The returned config carries no api_key or skill path;
// the caller layers those on top and may override any field.
func Lookup(name string) (llm.ModelConfig, bool) {
	for _, p := range presets {
		if p.Name == name {
			return p, true
		}
	}
	return llm.ModelConfig{}, false
}

// presets is the built-in catalog, keyed by model name. Values mirror each
// provider's public API docs; re-check the linked page when refreshing a
// model — context length, base URL, and capabilities change between versions.
var presets = []llm.ModelConfig{
	// Source: https://api-docs.deepseek.com/zh-cn/quick_start/pricing
	{
		Name:          "deepseek-flash",
		BaseURL:       "https://api.deepseek.com",
		ContextWindow: 1_000_000,
		ImageEnabled:  true,
		API:           llm.OpenAI,
	},
	{
		Name:          "deepseek-v4-pro",
		BaseURL:       "https://api.deepseek.com",
		ContextWindow: 1_000_000,
		API:           llm.OpenAI,
	},
}
