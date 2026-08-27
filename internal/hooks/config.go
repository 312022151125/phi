package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type HookEvent string

const (
	EventPreToolUse          HookEvent = "PreToolUse"
	EventPostToolUse         HookEvent = "PostToolUse"
	EventPostToolUseFailure  HookEvent = "PostToolUseFailure"
	EventSessionStart        HookEvent = "SessionStart"
	EventSessionShutdown     HookEvent = "SessionShutdown"
	EventSessionBeforeSwitch HookEvent = "SessionBeforeSwitch"
	EventPostTurn            HookEvent = "PostTurn"
	EventCommand             HookEvent = "Command"
)

var knownEvents = map[HookEvent]struct{}{
	EventPreToolUse:          {},
	EventPostToolUse:         {},
	EventPostToolUseFailure:  {},
	EventSessionStart:        {},
	EventSessionShutdown:     {},
	EventSessionBeforeSwitch: {},
	EventPostTurn:            {},
	EventCommand:             {},
}

// PluginFileName is the manifest that lists hooks in a hooks directory
// (or a plugin subdirectory).
const PluginFileName = "plugin.json"

type pluginFile struct {
	Hooks map[HookEvent][]hookMatcherRaw `json:"hooks"`
}

type hookMatcherRaw struct {
	Matcher string           `json:"matcher,omitempty"`
	Hooks   []commandHookRaw `json:"hooks"`
}

type commandHookRaw struct {
	Type    string `json:"type"`              // always "command"
	Command string `json:"command"`           // shell command to execute
	Shell   string `json:"shell,omitempty"`   // "bash" (default, uses $SHELL) | "powershell" (pwsh)
	Timeout int    `json:"timeout,omitempty"` // seconds, must be > 0
	Async   bool   `json:"async,omitempty"`   // run in background without blocking
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
	Command string
	Shell   string
	Timeout int
	Async   bool
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
	var raw pluginFile
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
	slices.Sort(events)

	hooks := make([]Hook, 0, len(events))
	for _, origEvent := range events {
		event := normalizeHookEvent(origEvent)
		if _, ok := knownEvents[event]; !ok {
			return nil, fmt.Errorf("hooks: %s: unknown event %q", abs, origEvent)
		}
		matchers, err := toHookMatchers(event, raw.Hooks[origEvent])
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

func toHookMatchers(event HookEvent, raw []hookMatcherRaw) ([]HookMatcher, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty matcher list")
	}
	matchers := make([]HookMatcher, 0, len(raw))
	for i, m := range raw {
		if event == EventCommand && strings.TrimSpace(m.Matcher) == "" {
			return nil, fmt.Errorf("matcher[%d]: Command hooks require matcher (slash command name)", i)
		}
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
		return nil, errors.New("empty hooks list")
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

// normalizeHookEvent maps legacy/CC names to Phi event names.
func normalizeHookEvent(e HookEvent) HookEvent {
	if e == "SessionEnd" {
		// CC SessionEnd implies termination; Phi fires this when leaving/switching
		// the active session (new / resume / quit), not when a session file is destroyed.
		return EventSessionShutdown
	}
	return e
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
		return CommandHook{}, errors.New("empty command")
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

	return CommandHook{
		Command: command,
		Shell:   shell,
		Timeout: h.Timeout,
		Async:   h.Async,
	}, nil
}
