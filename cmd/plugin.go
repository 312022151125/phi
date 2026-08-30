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
		Long: `Install a published Go extension (yaegi) from a GitHub repo into ~/.phi/extensions/<repo>/.

The repo root must contain index.go, or exactly one non-test *.go file.

Examples:
  phi plugin install alice/greet
  phi plugin install alice/greet@v1.2.3
  phi plugin install github.com/alice/greet@main

Security: extensions run with your full process permissions.`,
	}

	pluginInstallCommand = cli.Command{
		Name:    "install",
		ArgsUse: "<github-repo[@tag]>",
		Desc:    "git clone a GitHub repo into ~/.phi/extensions",
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
