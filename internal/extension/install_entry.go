package extension

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// findInstallEntry verifies a cloned plugin looks like a PXB extension.
func findInstallEntry(dir string) (string, error) {
	m, err := ReadManifest(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New(
				"cloned repo has no phi.yaml (PXB extensions need phi.yaml + a compiled binary)",
			)
		}
		return "", err
	}
	execPath := m.Exec
	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(dir, execPath)
	}
	if st, err := os.Stat(execPath); err != nil {
		return "", fmt.Errorf(
			"phi.yaml exec %q not found (build the binary before install, or ship it in the repo): %w",
			m.Exec,
			err,
		)
	} else if st.IsDir() {
		return "", fmt.Errorf("phi.yaml exec %q is a directory", m.Exec)
	}
	return execPath, nil
}
