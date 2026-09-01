package extension_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/extension"
)

func TestParseSpec(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		owner   string
		repo    string
		ref     string
		wantErr bool
	}{
		{in: "alice/greet", owner: "alice", repo: "greet"},
		{in: "alice/greet@v1.2.3", owner: "alice", repo: "greet", ref: "v1.2.3"},
		{in: "github.com/alice/greet", owner: "alice", repo: "greet"},
		{in: "github.com/alice/greet@main", owner: "alice", repo: "greet", ref: "main"},
		{in: "https://github.com/alice/greet", owner: "alice", repo: "greet"},
		{in: "https://github.com/alice/greet.git", owner: "alice", repo: "greet"},
		{in: "https://github.com/alice/greet@v1", owner: "alice", repo: "greet", ref: "v1"},
		{in: "https://github.com/alice/greet.git@v1", owner: "alice", repo: "greet", ref: "v1"},
		{in: "", wantErr: true},
		{in: "just-repo", wantErr: true},
		{in: "git@github.com:alice/greet.git", wantErr: true},
		{in: "https://gitlab.com/alice/greet", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := extension.ParseSpec(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.owner, got.Owner)
			assert.Equal(t, tc.repo, got.Repo)
			assert.Equal(t, tc.ref, got.Ref)
			assert.Equal(t, "https://github.com/"+tc.owner+"/"+tc.repo+".git", got.CloneURL())
			assert.Equal(t, tc.repo, got.ID())
		})
	}
}

func TestDiscoverIgnoresSubdirWithoutManifest(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "greet")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "greet.go"), []byte("package main\n"), 0o644))

	found, warns, err := extension.Discover(dir, "")
	require.NoError(t, err)
	assert.Empty(t, found)
	assert.Empty(t, warns)
}

func TestInstallClonesAndValidates(t *testing.T) {
	dir := t.TempDir()
	spec := extension.Spec{Owner: "alice", Repo: "greet", Ref: "v1"}
	var sawArgs []string
	err := extension.Install(t.Context(), extension.InstallOptions{
		Dir:  dir,
		Spec: spec,
		Git:  "git",
		RunGit: func(_ context.Context, gitBin string, args ...string) error {
			assert.Equal(t, "git", gitBin)
			sawArgs = append([]string{}, args...)
			dest := args[len(args)-1]
			require.NoError(t, os.MkdirAll(dest, 0o755))
			require.NoError(
				t,
				os.WriteFile(filepath.Join(dest, "phi.yaml"), []byte("name: greet\nexec: ./greet\n"), 0o644),
			)
			return os.WriteFile(filepath.Join(dest, "greet"), []byte("#!/bin/true\n"), 0o755)
		},
	})
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{"clone", "--depth", "1", "--branch", "v1", spec.CloneURL(), filepath.Join(dir, "greet")},
		sawArgs,
	)

	found, _, err := extension.Discover(dir, "")
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "greet", found[0].ID)
}

func TestInstallRejectsMissingEntry(t *testing.T) {
	dir := t.TempDir()
	err := extension.Install(t.Context(), extension.InstallOptions{
		Dir:  dir,
		Spec: extension.Spec{Owner: "alice", Repo: "empty"},
		Git:  "git",
		RunGit: func(_ context.Context, _ string, args ...string) error {
			dest := args[len(args)-1]
			return os.MkdirAll(dest, 0o755)
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "phi.yaml")
	_, err = os.Stat(filepath.Join(dir, "empty"))
	assert.True(t, os.IsNotExist(err), "failed install should clean up dest")
}

func TestInstallRejectsExisting(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "greet")
	require.NoError(t, os.Mkdir(dest, 0o755))
	err := extension.Install(t.Context(), extension.InstallOptions{
		Dir:  dir,
		Spec: extension.Spec{Owner: "alice", Repo: "greet"},
		Git:  "git",
		RunGit: func(context.Context, string, ...string) error {
			t.Fatal("git should not run")
			return nil
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
