package v1

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source labels where a discovered plugin came from.
const (
	SourceUser    = "user"
	SourceProject = "project"
)

// EnvHooks is the environment variable that disables hooks.
// Value "off" (case-insensitive) skips discovery entirely.
const EnvHooks = "PHI_HOOKS"

// Warning is a non-fatal discovery problem (bad plugin.json, unreadable dir).
type Warning struct {
	Path    string
	Message string
}

func (w Warning) String() string {
	if w.Path == "" {
		return w.Message
	}
	return w.Path + ": " + w.Message
}

// Discovered is one plugin.json worth of hooks from a discovery root.
type Discovered struct {
	Plugin string // plugin id (directory name, or "root" for hooksDir/plugin.json)
	Path   string // absolute path to plugin.json
	Hooks  []Hook
	Source string // SourceUser or SourceProject
}

// HooksDisabled reports whether PHI_HOOKS=off.
func HooksDisabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvHooks))
	return strings.EqualFold(v, "off")
}

// Discover loads plugin.json from userDir then projectDir.
// Same Plugin id: project replaces user (whole-file shadow).
// Missing directories are fine. Parse errors become Warnings; only unexpected
// I/O on a present directory returns err.
//
// Layout: <hooksDir>/plugin.json and <hooksDir>/<plugin>/plugin.json.
func Discover(userDir, projectDir string) ([]Discovered, []Warning, error) {
	if HooksDisabled() {
		return nil, nil, nil
	}

	byPlugin := make(map[string]Discovered)
	var warnings []Warning

	load := func(dir, source string) error {
		if dir == "" {
			return nil
		}
		found, warns, err := scanHooksDir(dir, source)
		warnings = append(warnings, warns...)
		if err != nil {
			return err
		}
		for _, d := range found {
			byPlugin[d.Plugin] = d
		}
		return nil
	}

	if err := load(userDir, SourceUser); err != nil {
		return nil, warnings, err
	}
	if err := load(projectDir, SourceProject); err != nil {
		return nil, warnings, err
	}

	out := make([]Discovered, 0, len(byPlugin))
	for _, d := range byPlugin {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Plugin < out[j].Plugin })
	return out, warnings, nil
}

func scanHooksDir(dir, source string) ([]Discovered, []Warning, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("hooks: read dir %s: %w", dir, err)
	}

	var (
		out      []Discovered
		warnings []Warning
		seen     = make(map[string]string) // plugin id → plugin path
	)

	loadFile := func(pluginID, pluginPath string) {
		d, warns := loadPluginFile(pluginID, pluginPath, source, seen)
		warnings = append(warnings, warns...)
		if d != nil {
			out = append(out, *d)
		}
	}

	rootPlugin := filepath.Join(dir, PluginFileName)
	if st, err := os.Stat(rootPlugin); err == nil && !st.IsDir() {
		loadFile("root", rootPlugin)
	} else if err != nil && !os.IsNotExist(err) {
		warnings = append(warnings, Warning{Path: rootPlugin, Message: err.Error()})
	}

	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		pluginPath := filepath.Join(dir, ent.Name(), PluginFileName)
		st, err := os.Stat(pluginPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			warnings = append(warnings, Warning{Path: pluginPath, Message: err.Error()})
			continue
		}
		if st.IsDir() {
			continue
		}
		loadFile(ent.Name(), pluginPath)
	}
	return out, warnings, nil
}

func loadPluginFile(pluginID, pluginPath, source string, seen map[string]string) (*Discovered, []Warning) {
	if prev, dup := seen[pluginID]; dup {
		return nil, []Warning{{
			Path:    pluginPath,
			Message: fmt.Sprintf("duplicate plugin %q (already defined in %s); skipped", pluginID, prev),
		}}
	}

	hooks, err := ParsePlugin(pluginPath)
	if err != nil {
		return nil, []Warning{{Path: pluginPath, Message: err.Error()}}
	}

	seen[pluginID] = pluginPath
	return &Discovered{
		Plugin: pluginID,
		Path:   pluginPath,
		Hooks:  hooks,
		Source: source,
	}, nil
}

// FormatDiscovered returns a one-line status for palette / logs.
func FormatDiscovered(d Discovered) string {
	events := make([]string, 0, len(d.Hooks))
	n := 0
	for _, h := range d.Hooks {
		events = append(events, string(h.Event))
		for _, m := range h.Matchers {
			n += len(m.Hooks)
		}
	}
	return fmt.Sprintf("%s  events=%s  commands=%d  [%s]", d.Plugin, strings.Join(events, ","), n, d.Source)
}
