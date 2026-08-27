package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerPreToolDenyExit2(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "deny.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 2\n"), 0o644))
	require.NoError(t, os.Chmod(script, 0o755))

	pluginPath := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(pluginPath, []byte(`{
  "hooks": {
    "PreToolUse": [{
      "matcher": "bash",
      "hooks": [{ "command": "./deny.sh" }]
    }]
  }
}`), 0o644))

	mgr, err := loadManagerFromPlugin(t, pluginPath)
	require.NoError(t, err)

	out := mgr.PreTool(t.Context(), ToolEvent{
		SessionID: "s1",
		Cwd:       dir,
		Tool:      "bash",
		ToolUseID: "tu1",
		Input:     json.RawMessage(`{"command":"echo hi"}`),
	})
	assert.True(t, out.Denied)
}

func TestManagerPreToolUpdatedInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "modify.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
cat >/dev/null
printf '%s' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"echo safe"}}}'
`), 0o644))
	require.NoError(t, os.Chmod(script, 0o755))

	pluginPath := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(pluginPath, []byte(`{
  "hooks": {
    "PreToolUse": [{
      "hooks": [{ "command": "./modify.sh" }]
    }]
  }
}`), 0o644))

	mgr, err := loadManagerFromPlugin(t, pluginPath)
	require.NoError(t, err)

	out := mgr.PreTool(t.Context(), ToolEvent{
		Cwd:   dir,
		Tool:  "bash",
		Input: json.RawMessage(`{"command":"echo hi"}`),
	})
	assert.False(t, out.Denied)
	assert.JSONEq(t, `{"command":"echo safe"}`, string(out.Input))
}

func TestManagerPostToolFailureEvent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "post.sh")
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
cat >/dev/null
printf '%s' '{"hookSpecificOutput":{"hookEventName":"PostToolUseFailure","additionalContext":"saw failure"}}'
`), 0o644))
	require.NoError(t, os.Chmod(script, 0o755))

	pluginPath := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(pluginPath, []byte(`{
  "hooks": {
    "PostToolUseFailure": [{
      "hooks": [{ "command": "./post.sh" }]
    }]
  }
}`), 0o644))

	mgr, err := loadManagerFromPlugin(t, pluginPath)
	require.NoError(t, err)

	out := mgr.PostTool(t.Context(), ToolEvent{
		Cwd:  dir,
		Tool: "bash",
		Err:  "boom",
	})
	assert.Equal(t, "saw failure", out.Context)
}

func TestMatchesPattern(t *testing.T) {
	assert.True(t, matchesPattern("", "bash"))
	assert.True(t, matchesPattern("Write|Edit", "Edit"))
	assert.True(t, matchesPattern("^bash$", "bash"))
	assert.False(t, matchesPattern("Write", "bash"))
}

func TestLoadIntegration(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "policy")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, PluginFileName), []byte(`{
  "hooks": {
    "SessionStart": [{
      "matcher": "startup",
      "hooks": [{ "command": "true" }]
    }]
  }
}`), 0o644))

	mgr, warns, err := Load(dir, "")
	require.NoError(t, err)
	assert.Empty(t, warns)
	require.NotNil(t, mgr)
	out := mgr.SessionStart(t.Context(), SessionEvent{Reason: "startup"})
	assert.Empty(t, out.Toast)
}

func TestManagerRunCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "cmd.sh")
	require.NoError(
		t,
		os.WriteFile(script, []byte("#!/bin/sh\ncat >/dev/null\necho '{\"submit\":\"hello from hook\"}'\n"), 0o644),
	)
	require.NoError(t, os.Chmod(script, 0o755))
	pluginPath := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(pluginPath, []byte(`{
  "hooks": {
    "Command": [{
      "matcher": "review",
      "hooks": [{ "command": "./cmd.sh" }]
    }]
  }
}`), 0o644))

	mgr, err := loadManagerFromPlugin(t, pluginPath)
	require.NoError(t, err)
	require.Len(t, mgr.CommandEntries(), 1)
	assert.Equal(t, "review", mgr.CommandEntries()[0].Name)

	res, err := mgr.RunCommand(t.Context(), "review", CommandEvent{Cwd: dir, Args: []string{"a", "b"}})
	require.NoError(t, err)
	assert.Equal(t, "hello from hook", res.Submit)
}

func TestManagerSessionBeforeSwitchDeny(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "deny.sh")
	require.NoError(
		t,
		os.WriteFile(
			script,
			[]byte("#!/bin/sh\ncat >/dev/null\necho '{\"action\":\"deny\",\"reason\":\"dirty repo\"}'\n"),
			0o644,
		),
	)
	require.NoError(t, os.Chmod(script, 0o755))
	pluginPath := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(pluginPath, []byte(`{
  "hooks": {
    "SessionBeforeSwitch": [{
      "hooks": [{ "command": "./deny.sh" }]
    }]
  }
}`), 0o644))

	mgr, err := loadManagerFromPlugin(t, pluginPath)
	require.NoError(t, err)
	out := mgr.SessionBeforeSwitch(t.Context(), SessionEvent{Reason: "new"})
	assert.True(t, out.Denied)
	assert.Equal(t, "dirty repo", out.Reason)
}

func TestManagerPostTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell hook fixture")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, ".post-turn")
	script := filepath.Join(dir, "audit.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\ncat >/dev/null\ntouch .post-turn\n"), 0o644))
	require.NoError(t, os.Chmod(script, 0o755))
	pluginPath := filepath.Join(dir, PluginFileName)
	require.NoError(t, os.WriteFile(pluginPath, []byte(`{
  "hooks": {
    "PostTurn": [{
      "hooks": [{ "command": "./audit.sh" }]
    }]
  }
}`), 0o644))

	mgr, err := loadManagerFromPlugin(t, pluginPath)
	require.NoError(t, err)
	mgr.PostTurn(t.Context(), SessionEvent{
		SessionID: "s1",
		Cwd:       dir,
		MessageID: "m1",
		Usage:     SessionUsage{TotalTokens: 42},
	})
	_, err = os.Stat(marker)
	assert.NoError(t, err)
}

func loadManagerFromPlugin(t *testing.T, pluginPath string) (*Manager, error) {
	t.Helper()
	hookList, err := ParsePlugin(pluginPath)
	if err != nil {
		return nil, err
	}
	return NewManager(Discovered{Plugin: "test", Path: pluginPath, Hooks: hookList}), nil
}
