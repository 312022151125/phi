package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pulseaiclub/phi/internal/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProject discovers a project under a temp HOME so tests never touch the
// real ~/.phi, and returns a project plus a PATH dir for binary stubs.
func testProject(t *testing.T) (*project.Project, string) {
	t.Helper()
	home := t.TempDir()
	pathDir := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", pathDir)

	p, err := project.Discover("")
	require.NoError(t, err)
	return p, pathDir
}

func TestShouldBootstrapWhenMissing(t *testing.T) {
	p, _ := testProject(t)
	// Empty bin dir and empty PATH dir → must download.
	assert.True(t, shouldBootstrap(p, "fd"))
	assert.True(t, shouldBootstrap(p, "rg"))
}

func TestShouldBootstrapWhenInBinDir(t *testing.T) {
	p, _ := testProject(t)
	binName := "fd"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	require.NoError(t, os.WriteFile(filepath.Join(p.Global().BinDir(), binName), []byte("x"), 0o755))

	assert.False(t, shouldBootstrap(p, "fd"))
	assert.True(t, shouldBootstrap(p, "rg"))
}

func TestShouldBootstrapWhenOnPATH(t *testing.T) {
	p, pathDir := testProject(t)
	binName := "rg"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	require.NoError(t, os.WriteFile(filepath.Join(pathDir, binName), []byte("x"), 0o755))

	assert.False(t, shouldBootstrap(p, "rg"))
	assert.True(t, shouldBootstrap(p, "fd"))
}
