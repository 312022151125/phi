package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
)

// Load resolves model config from ~/.phi/config.yaml,
// then applies environment overrides.
//
// Env (any of):
//
//	PHI_API_KEY
//	PHI_BASE_URL
//	PHI_MODEL
//	PHI_SKILL_PATH
func Load() (llm.ModelConfig, error) {
	cfg := loadFile()
	applyEnv(&cfg)

	if cfg.APIKey == "" {
		return llm.ModelConfig{}, fmt.Errorf("missing api_key (set PHI_API_KEY or primary_model.api_key in ~/.phi/config.yaml)")
	}
	if cfg.Name == "" {
		return llm.ModelConfig{}, fmt.Errorf("missing model name (set PHI_MODEL or primary_model.name in config)")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.SkillPath == "" {
		cfg.SkillPath = filepath.Join(homeDir(), ".phi", "skills")
	}
	return cfg, nil
}

type fileModel struct {
	Name          string
	APIKey        string
	BaseURL       string
	SkillPath     string
	ContextWindow int
}

func loadFile() llm.ModelConfig {
	paths := []string{
		filepath.Join(homeDir(), ".phi", "config.yaml"),
	}
	for _, p := range paths {
		m, ok := parsePrimaryModelYAML(p)
		if !ok {
			continue
		}
		return llm.ModelConfig{
			Name:          m.Name,
			APIKey:        m.APIKey,
			BaseURL:       m.BaseURL,
			SkillPath:     m.SkillPath,
			ContextWindow: m.ContextWindow,
		}
	}
	return llm.ModelConfig{}
}

// parsePrimaryModelYAML reads primary_model.{name,api_key,base_url} and the
// top-level skill_path with a tiny line scanner so we don't need a YAML
// dependency.
func parsePrimaryModelYAML(path string) (fileModel, bool) {
	f, err := os.Open(path)
	if err != nil {
		return fileModel{}, false
	}
	defer f.Close()

	var (
		m       fileModel
		inBlock bool
	)
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
					m.SkillPath = val
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
			m.Name = val
		case "api_key":
			m.APIKey = val
		case "base_url":
			m.BaseURL = val
		case "context_window":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				m.ContextWindow = n
			}
		}
	}
	return m, m.Name != "" || m.APIKey != ""
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

func applyEnv(m *llm.ModelConfig) {
	if v := firstEnv("PHI_API_KEY"); v != "" {
		m.APIKey = v
	}
	if v := firstEnv("PHI_BASE_URL"); v != "" {
		m.BaseURL = v
	}
	if v := firstEnv("PHI_MODEL"); v != "" {
		m.Name = v
	}
	if v := firstEnv("PHI_SKILL_PATH"); v != "" {
		m.SkillPath = v
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}
