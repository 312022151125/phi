package v1

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePluginOK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(path, []byte(`{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "./guard.sh",
            "if": "Bash(git *)",
            "timeout": 30,
            "statusMessage": "checking",
            "once": true,
            "asyncRewake": true
          }
        ]
      }
    ],
    "SessionStart": [
      {
        "hooks": [{ "command": "echo hello" }]
      }
    ]
  }
}`), 0o644))

	hooks, err := ParsePlugin(path)
	require.NoError(t, err)
	require.Len(t, hooks, 2)

	assert.Equal(t, EventPreToolUse, hooks[0].Event)
	require.Len(t, hooks[0].Matchers, 1)
	assert.Equal(t, "Bash", hooks[0].Matchers[0].Matcher)
	require.Len(t, hooks[0].Matchers[0].Hooks, 1)

	cmd := hooks[0].Matchers[0].Hooks[0]
	assert.Equal(t, "command", cmd.Type)
	assert.Equal(t, "./guard.sh", cmd.Command)
	assert.Equal(t, "Bash(git *)", cmd.If)
	assert.Equal(t, 30, cmd.Timeout)
	assert.Equal(t, "checking", cmd.StatusMessage)
	assert.True(t, cmd.Once)
	assert.True(t, cmd.Async, "asyncRewake implies async")
	assert.True(t, cmd.AsyncRewake)
	assert.Equal(t, dir, hooks[0].Dir)

	assert.Equal(t, EventSessionStart, hooks[1].Event)
	assert.Equal(t, "echo hello", hooks[1].Matchers[0].Hooks[0].Command)
}

func TestParsePluginErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty", `{}`},
		{"unknown event", `{"hooks":{"Nope":[{"hooks":[{"command":"x"}]}]}}`},
		{"bad type", `{"hooks":{"PreToolUse":[{"hooks":[{"type":"prompt","command":"x"}]}]}}`},
		{"empty command", `{"hooks":{"PreToolUse":[{"hooks":[{"command":"  "}]}]}}`},
		{"bad shell", `{"hooks":{"PreToolUse":[{"hooks":[{"command":"x","shell":"zsh"}]}]}}`},
		{"neg timeout", `{"hooks":{"PreToolUse":[{"hooks":[{"command":"x","timeout":-1}]}]}}`},
		{"empty matchers", `{"hooks":{"PreToolUse":[]}}`},
		{"empty hooks", `{"hooks":{"PreToolUse":[{"hooks":[]}]}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), PluginFileName)
			require.NoError(t, os.WriteFile(path, []byte(tt.body), 0o644))
			_, err := ParsePlugin(path)
			assert.Error(t, err)
		})
	}
}

func TestDiscoverProjectShadowsUser(t *testing.T) {
	user := t.TempDir()
	project := t.TempDir()

	writePlugin := func(root, plugin, body string) {
		dir := filepath.Join(root, plugin)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, PluginFileName), []byte(body), 0o644))
	}

	userBody := `{"hooks":{"SessionStart":[{"hooks":[{"command":"echo user"}]}]}}`
	projectBody := `{"hooks":{"SessionStart":[{"hooks":[{"command":"echo project"}]}]}}`
	writePlugin(user, "guard", userBody)
	writePlugin(user, "audit", userBody)
	writePlugin(project, "guard", projectBody)

	found, warns, err := Discover(user, project)
	require.NoError(t, err)
	assert.Empty(t, warns)
	require.Len(t, found, 2)

	byPlugin := map[string]Discovered{}
	for _, d := range found {
		byPlugin[d.Plugin] = d
	}
	require.Contains(t, byPlugin, "guard")
	require.Contains(t, byPlugin, "audit")
	assert.Equal(t, SourceProject, byPlugin["guard"].Source)
	assert.Equal(t, "echo project", byPlugin["guard"].Hooks[0].Matchers[0].Hooks[0].Command)
	assert.Equal(t, SourceUser, byPlugin["audit"].Source)
}

func TestDiscoverDisabled(t *testing.T) {
	t.Setenv(EnvHooks, "off")
	found, warns, err := Discover(t.TempDir(), t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, found)
	assert.Nil(t, warns)
}
