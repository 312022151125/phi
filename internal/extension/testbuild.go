package extension

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Materialize writes a PXB extension module under dir and builds its binary.
// mainGo must be a complete package main that uses ext/sdk.
// dir becomes the extension directory (contains phi.yaml + binary).
func Materialize(dir, name, version, mainGo string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	phiRoot, err := moduleRoot()
	if err != nil {
		return err
	}
	mod := fmt.Sprintf(
		"module %s\n\ngo 1.26\n\nrequire github.com/pulseaiclub/phi v0.0.0\n\nreplace github.com/pulseaiclub/phi => %s\n",
		name,
		filepath.ToSlash(phiRoot),
	)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy: %w\n%s", err, out)
	}
	bin := "extbin"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd = exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build: %w\n%s", err, out)
	}
	yaml := fmt.Sprintf("name: %s\nversion: %q\nexec: ./%s\n", name, version, bin)
	return os.WriteFile(filepath.Join(dir, "phi.yaml"), []byte(yaml), 0o644)
}

func moduleRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("extension: runtime.Caller failed")
	}
	// internal/extension/testbuild.go → repo root
	return filepath.Abs(filepath.Join(filepath.Dir(file), "../.."))
}
