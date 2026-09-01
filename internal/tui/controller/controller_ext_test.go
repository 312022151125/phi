package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/extension"
	"github.com/pulseaiclub/phi/internal/project"
)

func TestSwapExtensionRunner_ClosesPrevious(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("PHI_MODEL", "test-model")
	t.Setenv("PHI_API_KEY", "test-key")
	t.Setenv("PHI_BASE_URL", "http://127.0.0.1:9")
	t.Setenv(extension.EnvExtensions, "off")

	cwd := t.TempDir()
	proj, err := project.Discover(cwd)
	require.NoError(t, err)
	require.NoError(t, proj.LoadConfig())

	ctrl, err := NewController(NewBus(nil), proj, cwd)
	require.NoError(t, err)

	first, _, err := extension.Load(t.TempDir(), "")
	require.NoError(t, err)
	require.NotNil(t, first)
	ctrl.swapExtensionRunner(first)
	assert.Same(t, first, ctrl.Extensions())

	second, _, err := extension.Load(t.TempDir(), "")
	require.NoError(t, err)
	require.NotNil(t, second)
	ctrl.swapExtensionRunner(second)
	assert.Same(t, second, ctrl.Extensions())
	// Previous runner must be closed; a second Close is a safe no-op.
	first.Close()

	ctrl.Close()
	assert.Nil(t, ctrl.Extensions())
	// Idempotent.
	ctrl.Close()
}
