package main

import (
	"context"
	"os"
	"time"

	cli "github.com/pulseaiclub/pli"

	"github.com/pulseaiclub/phi/internal/extension"
	"github.com/pulseaiclub/phi/internal/project"
)

var (
	pluginCommand = cli.Command{
		Name: "plugin",
		Desc: "manage extensions (install from GitHub)",
		Long: `Install a PXB extension from a GitHub repo into ~/.phi/extensions/<repo>/.

Prefers a GitHub Release archive for this OS/arch (same layout as phi update):

  {repo}_{version}_{goos}_{goarch}.tar.gz   # .zip on Windows

The archive must contain phi.yaml and the compiled exec binary. If no matching
release asset exists, falls back to a shallow git clone (repo must already ship
the binary).

Examples:
  phi plugin install alice/greet
  phi plugin install alice/greet@v1.2.3
  phi plugin install github.com/alice/greet@main

Security: extension processes run with your full permissions.`,
	}

	pluginInstallCommand = cli.Command{
		Name:    "install",
		ArgsUse: "<github-repo[@tag]>",
		Desc:    "install an extension from a GitHub release (git clone fallback)",
	}
)

func init() {
	pluginInstallCommand.Run = func(args []string, _ cli.Flags) error {
		return pluginInstall(args)
	}
	pluginCommand.Add(&pluginInstallCommand)
}

func pluginInstall(args []string) error {
	if len(args) != 1 {
		return pluginInstallCommand.Usagef("expected <github-repo[@tag]>")
	}
	spec, err := extension.ParseSpec(args[0])
	if err != nil {
		return err
	}
	proj := project.GetDefaultProject()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return extension.Install(ctx, extension.InstallOptions{
		Dir:    proj.Global().ExtensionsDir(),
		Spec:   spec,
		Stdout: os.Stdout,
	})
}
