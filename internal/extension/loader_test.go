package extension_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/extension"
)

func TestDiscoverGoFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "index.go"), []byte("package main\n"), 0o644))

	found, warns, err := extension.Discover(dir, "")
	require.NoError(t, err)
	assert.Empty(t, warns)
	require.Len(t, found, 2)
	ids := []string{found[0].ID, found[1].ID}
	assert.Contains(t, ids, "hello")
	assert.Contains(t, ids, "nested")
}

func TestExtensionsDisabled(t *testing.T) {
	t.Setenv(extension.EnvExtensions, "off")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n"), 0o644))
	found, _, err := extension.Discover(dir, "")
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestLoadAndPreToolBlock(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import (
	"encoding/json"
	"strings"
	"github.com/pulseaiclub/phi/ext"
)

func Extension(phi *ext.API) {
	phi.On(ext.EventToolCall, func(ev ext.ToolCallEvent, ctx *ext.Context) *ext.ToolCallResult {
		if ev.ToolName != "bash" {
			return nil
		}
		var in struct {
			Command string ` + "`json:\"command\"`" + `
		}
		_ = json.Unmarshal(ev.Input, &in)
		if strings.Contains(in.Command, "phi-deny") {
			return &ext.ToolCallResult{Block: true, Reason: "blocked by extension"}
		}
		return nil
	})
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "guard.go"), []byte(src), 0o644))

	r, warns, err := extension.Load(dir, "")
	require.NoError(t, err)
	if len(warns) > 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	require.NotNil(t, r)
	require.Len(t, r.Loaded(), 1)

	input := json.RawMessage(`{"command":"echo phi-deny"}`)
	_, blocked, reason, _ := r.PreTool(t.Context(), "bash", "1", input)
	assert.True(t, blocked)
	assert.Equal(t, "blocked by extension", reason)

	_, blocked, _, _ = r.PreTool(t.Context(), "bash", "2", json.RawMessage(`{"command":"echo ok"}`))
	assert.False(t, blocked)
}

func TestRegisterTool(t *testing.T) {
	dir := t.TempDir()
	src := `package main

import (
	"context"
	"encoding/json"
	"github.com/pulseaiclub/phi/ext"
)

func Extension(phi *ext.API) {
	phi.RegisterTool(ext.ToolDef{
		Name:        "greet",
		Description: "Greet someone",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
		Execute: func(ctx context.Context, args json.RawMessage) (ext.ToolResult, error) {
			var in struct {
				Name string ` + "`json:\"name\"`" + `
			}
			_ = json.Unmarshal(args, &in)
			return ext.ToolResult{Content: "Hello, " + in.Name + "!"}, nil
		},
	})
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "greet.go"), []byte(src), 0o644))
	r, warns, err := extension.Load(dir, "")
	require.NoError(t, err)
	require.Empty(t, warns)
	tools := r.ExtensionTools()
	require.Len(t, tools, 1)
	assert.Equal(t, "greet", tools[0].Definition.Name)

	res, err := tools[0].Run(t.Context(), json.RawMessage(`{"name":"phi"}`))
	require.NoError(t, err)
	assert.Equal(t, "Hello, phi!", res.Content)
}
