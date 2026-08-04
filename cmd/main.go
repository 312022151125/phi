package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/app"
	"github.com/pulseaiclub/phi/internal/project"
	"github.com/pulseaiclub/phi/internal/tui"
	"github.com/pulseaiclub/xui"
)

func main() {
	proj := project.GetDefaultProject()
	if err := proj.LoadConfig(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	cfg := proj.Config().Model()

	if err := EnsureSearchTools(context.Background(), proj); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not install search tools:", err)
	}

	vx, err := xui.New(xui.Options{Mouse: true, BracketedPaste: true})
	if err != nil {
		panic(err)
	}
	defer func(vx *xui.XUI) {
		err := vx.Close()
		if err != nil {
			panic(err)
		}
	}(vx)

	cwd, _ := os.Getwd()
	th := components.DefaultTheme()
	m := tui.NewEditor(vx, th, cwd, cfg.Name, cfg.SkillPath, cfg.ContextWindow)

	app := app.NewApp(vx)
	app.Anim = true
	m.App = app
	if err := app.Run(m); err != nil {
		panic(err)
	}
}
