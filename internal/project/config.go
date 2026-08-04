package project

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
)

// Config is the project-level configuration loaded from ~/.phi/config.yaml.
// It keeps the primary model separate from agent-wide settings such as the
// skill directory (mirroring panda's project.Config).
type Config struct {
	PrimaryModel llm.ModelConfig
	SkillPath    string
}

// Model returns the primary model config with the skill path applied, ready
// for agent.NewEngine.
func (c *Config) Model() llm.ModelConfig {
	m := c.PrimaryModel
	if m.SkillPath == "" {
		m.SkillPath = c.SkillPath
	}
	return m
}

// loadConfig reads the config file, applies environment overrides, and fills
// in defaults. A missing file yields a zero Config so env-only setups work.
func loadConfig(global GlobalLayout) (*Config, error) {
	cfg := parseConfigFile(global.ConfigFile())
	applyEnvOverrides(cfg)

	if cfg.PrimaryModel.APIKey == "" {
		return nil, fmt.Errorf("missing api_key (set PHI_API_KEY or primary_model.api_key in %s)", global.ConfigFile())
	}
	if cfg.PrimaryModel.Name == "" {
		return nil, fmt.Errorf("missing model name (set PHI_MODEL or primary_model.name in %s)", global.ConfigFile())
	}
	if cfg.PrimaryModel.BaseURL == "" {
		cfg.PrimaryModel.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.SkillPath == "" {
		cfg.SkillPath = global.SkillsDir()
	}
	return cfg, nil
}

// parseConfigFile reads primary_model.{name,api_key,base_url,context_window}
// and the top-level skill_path with a tiny line scanner so we don't need a
// YAML dependency. A missing or unreadable file returns a zero Config.
func parseConfigFile(path string) *Config {
	cfg := &Config{}
	f, err := os.Open(path)
	if err != nil {
		return cfg
	}
	defer f.Close()

	var inBlock bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Top-level key
			if strings.HasPrefix(trimmed, "skill_path:") {
				_, val, ok := splitYAMLKV(trimmed)
				if ok {
					cfg.SkillPath = val
				}
			}
			inBlock = strings.HasPrefix(trimmed, "primary_model:")
			continue
		}
		if !inBlock {
			continue
		}
		key, val, ok := splitYAMLKV(trimmed)
		if !ok {
			continue
		}
		switch key {
		case "name":
			cfg.PrimaryModel.Name = val
		case "api_key":
			cfg.PrimaryModel.APIKey = val
		case "base_url":
			cfg.PrimaryModel.BaseURL = val
		case "context_window":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.PrimaryModel.ContextWindow = n
			}
		}
	}
	return cfg
}

func applyEnvOverrides(c *Config) {
	if v := firstEnv("PHI_API_KEY"); v != "" {
		c.PrimaryModel.APIKey = v
	}
	if v := firstEnv("PHI_BASE_URL"); v != "" {
		c.PrimaryModel.BaseURL = v
	}
	if v := firstEnv("PHI_MODEL"); v != "" {
		c.PrimaryModel.Name = v
	}
	if v := firstEnv("PHI_SKILL_PATH"); v != "" {
		c.SkillPath = v
	}
}

func splitYAMLKV(line string) (key, val string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:i])
	val = strings.TrimSpace(line[i+1:])
	val = strings.Trim(val, `"'`)
	return key, val, key != ""
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
