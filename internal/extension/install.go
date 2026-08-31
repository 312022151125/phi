package extension

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallOptions configures Install.
type InstallOptions struct {
	// Dir is the extensions directory (typically ~/.phi/extensions).
	Dir string
	// Spec is the GitHub plugin to install.
	Spec   Spec
	Stdout io.Writer
	// Git, if set, overrides the git executable lookup (tests).
	Git string
	// RunGit, if set, replaces the real git invocation (tests).
	RunGit func(ctx context.Context, gitBin string, args ...string) error
}

func (o InstallOptions) out() io.Writer {
	if o.Stdout != nil {
		return o.Stdout
	}
	return io.Discard
}

// Install clones a GitHub repo into Dir/<repo>/ and verifies it looks like an
// extension (index.go or a single *.go at the repo root).
func Install(ctx context.Context, opts InstallOptions) error {
	if opts.Dir == "" {
		return errors.New("extensions directory is required")
	}
	if opts.Spec.Owner == "" || opts.Spec.Repo == "" {
		return errors.New("invalid plugin spec: missing owner/repo")
	}

	dest := filepath.Join(opts.Dir, opts.Spec.ID())
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin %q already exists at %s (remove it first)", opts.Spec.ID(), dest)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dest, err)
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return fmt.Errorf("create extensions dir %s: %w", opts.Dir, err)
	}

	gitBin := opts.Git
	if gitBin == "" {
		var err error
		gitBin, err = exec.LookPath("git")
		if err != nil {
			return errors.New("git not found in PATH (required to install plugins)")
		}
	}

	args := []string{"clone", "--depth", "1"}
	if opts.Spec.Ref != "" {
		args = append(args, "--branch", opts.Spec.Ref)
	}
	args = append(args, opts.Spec.CloneURL(), dest)

	run := opts.RunGit
	if run == nil {
		run = defaultRunGit
	}
	if err := run(ctx, gitBin, args...); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}

	entry, err := findInstallEntry(dest)
	if err != nil {
		_ = os.RemoveAll(dest)
		return err
	}

	refNote := "default branch"
	if opts.Spec.Ref != "" {
		refNote = opts.Spec.Ref
	}
	_, _ = fmt.Fprintf(opts.out(), "installed %s/%s (%s) → %s\n", opts.Spec.Owner, opts.Spec.Repo, refNote, dest)
	_, _ = fmt.Fprintf(opts.out(), "entry: %s\n", entry)
	_, _ = fmt.Fprintln(
		opts.out(),
		"warning: extensions run with your full process permissions; only install from sources you trust",
	)
	_, _ = fmt.Fprintln(opts.out(), "reload: Ctrl+K → extensions → reload (or restart phi)")
	return nil
}

func defaultRunGit(ctx context.Context, gitBin string, args ...string) error {
	cmd := exec.CommandContext(ctx, gitBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}
