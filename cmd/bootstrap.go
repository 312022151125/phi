package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pulseaiclub/phi/internal/project"
	"github.com/pulseaiclub/phi/internal/toolmanager"
)

const bootstrapDownloadTimeout = 5 * time.Minute

// EnsureSearchTools installs fd and ripgrep into the phi bin dir
// (~/.phi/bin) when they are missing from both the bin dir and PATH.
// Failures are non-fatal: the search tools fall back to PATH at runtime
// and report a clear error if truly unavailable.
func EnsureSearchTools(ctx context.Context, proj *project.Project) error {
	for _, tool := range []string{"fd", "rg"} {
		if !shouldBootstrap(proj, tool) {
			continue
		}
		dlCtx, cancel := context.WithTimeout(ctx, bootstrapDownloadTimeout)
		_, err := toolmanager.DownloadTool(dlCtx, tool)
		cancel()
		if err != nil {
			return fmt.Errorf("%s: %w", tool, err)
		}
	}
	return nil
}

// shouldBootstrap is true when the tool binary is missing from the phi bin
// dir and from PATH, i.e. it needs a download. This mirrors panda's
// fileutil.ShouldBootstrapSearchTool.
func shouldBootstrap(proj *project.Project, name string) bool {
	binName := name
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	if _, err := os.Stat(filepath.Join(proj.Global().BinDir(), binName)); err == nil {
		return false
	}
	if _, err := exec.LookPath(binName); err == nil {
		return false
	}
	return true
}
