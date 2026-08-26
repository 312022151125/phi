package v1

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type HookEvent string

const (
	EventPreToolUse         HookEvent = "PreToolUse"
	EventPostToolUse        HookEvent = "PostToolUse"
	EventPostToolUseFailure HookEvent = "PostToolUseFailure"
	EventSessionStart       HookEvent = "SessionStart"
	EventSessionEnd         HookEvent = "SessionEnd"
)

var knownEvents = map[HookEvent]struct{}{
	EventPreToolUse:         {},
	EventPostToolUse:        {},
	EventPostToolUseFailure: {},
	EventSessionStart:       {},
	EventSessionEnd:         {},
}

// PluginFileName is the manifest that lists hooks in a hooks directory
// (or a plugin subdirectory).
const PluginFileName = "plugin.json"

type settingsFile struct {
	Hooks map[HookEvent][]hookMatcherRaw `json:"hooks"`
}

type hookMatcherRaw struct {
	Matcher string           `json:"matcher,omitempty"`
	Hooks   []commandHookRaw `json:"hooks"`
}

type commandHookRaw struct {
	Type          string `json:"type"`                    // always "command"
	Command       string `json:"command"`                 // shell command to execute
	If            string `json:"if,omitempty"`            // permission-rule filter, e.g. "Bash(git *)"
	Shell         string `json:"shell,omitempty"`         // "bash" (default, uses $SHELL) | "powershell" (pwsh)
	Timeout       int    `json:"timeout,omitempty"`       // seconds, must be > 0
	StatusMessage string `json:"statusMessage,omitempty"` // spinner text while running
	Once          bool   `json:"once,omitempty"`          // run once, then removed
	Async         bool   `json:"async,omitempty"`         // run in background without blocking
	AsyncRewake   bool   `json:"asyncRewake,omitempty"`   // background; exit 2 wakes the model (implies async)
}

// Hook is one event's matchers from a plugin.json.
type Hook struct {
	Path     string // absolute path to plugin.json
	Dir      string // directory containing plugin.json (cwd for relative commands)
	Event    HookEvent
	Matchers []HookMatcher
}

type HookMatcher struct {
	Matcher string
	Hooks   []CommandHook
}

// CommandHook is a shell command hook. Command is kept as written; relative
// paths are resolved at exec time against Dir (shell cwd), not Abs'd here—
// the value may include args (e.g. "./lint.sh --strict").
type CommandHook struct {
	Type          string
	Command       string
	If            string
	Shell         string
	Timeout       int
	StatusMessage string
	Once          bool
	Async         bool
	AsyncRewake   bool
}

// ParsePlugin reads a hooks plugin.json and returns one Hook per event.
func ParsePlugin(path string) ([]Hook, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("hooks: resolve plugin path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("hooks: read %s: %w", abs, err)
	}
	return parsePluginBytes(abs, data)
}

func parsePluginBytes(abs string, data []byte) ([]Hook, error) {
	var raw settingsFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("hooks: parse %s: %w", abs, err)
	}
	if len(raw.Hooks) == 0 {
		return nil, fmt.Errorf("hooks: %s: missing hooks (want a non-empty \"hooks\" object)", abs)
	}

	dir := filepath.Dir(abs)
	events := make([]HookEvent, 0, len(raw.Hooks))
	for event := range raw.Hooks {
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i] < events[j] })

	hooks := make([]Hook, 0, len(events))
	for _, event := range events {
		if _, ok := knownEvents[event]; !ok {
			return nil, fmt.Errorf("hooks: %s: unknown event %q", abs, event)
		}
		matchers, err := toHookMatchers(raw.Hooks[event])
		if err != nil {
			return nil, fmt.Errorf("hooks: %s: %s: %w", abs, event, err)
		}
		hooks = append(hooks, Hook{
			Path:     abs,
			Dir:      dir,
			Event:    event,
			Matchers: matchers,
		})
	}
	return hooks, nil
}

func toHookMatchers(raw []hookMatcherRaw) ([]HookMatcher, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty matcher list")
	}
	matchers := make([]HookMatcher, 0, len(raw))
	for i, m := range raw {
		cmds, err := toCommandHooks(m.Hooks)
		if err != nil {
			return nil, fmt.Errorf("matcher[%d]: %w", i, err)
		}
		matchers = append(matchers, HookMatcher{
			Matcher: m.Matcher,
			Hooks:   cmds,
		})
	}
	return matchers, nil
}

func toCommandHooks(raw []commandHookRaw) ([]CommandHook, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty hooks list")
	}
	out := make([]CommandHook, 0, len(raw))
	for i, h := range raw {
		cmd, err := normalizeCommandHook(h)
		if err != nil {
			return nil, fmt.Errorf("hooks[%d]: %w", i, err)
		}
		out = append(out, cmd)
	}
	return out, nil
}

func normalizeCommandHook(h commandHookRaw) (CommandHook, error) {
	typ := strings.TrimSpace(h.Type)
	if typ == "" {
		typ = "command"
	}
	if typ != "command" {
		return CommandHook{}, fmt.Errorf("unsupported hook type %q (only \"command\")", typ)
	}

	command := strings.TrimSpace(h.Command)
	if command == "" {
		return CommandHook{}, fmt.Errorf("empty command")
	}

	shell := strings.TrimSpace(h.Shell)
	switch shell {
	case "", "bash", "powershell", "pwsh":
	default:
		return CommandHook{}, fmt.Errorf("unsupported shell %q (want bash, powershell, or pwsh)", shell)
	}

	if h.Timeout < 0 {
		return CommandHook{}, fmt.Errorf("timeout must be > 0, got %d", h.Timeout)
	}

	async := h.Async || h.AsyncRewake
	return CommandHook{
		Type:          typ,
		Command:       command,
		If:            strings.TrimSpace(h.If),
		Shell:         shell,
		Timeout:       h.Timeout,
		StatusMessage: strings.TrimSpace(h.StatusMessage),
		Once:          h.Once,
		Async:         async,
		AsyncRewake:   h.AsyncRewake,
	}, nil
}
