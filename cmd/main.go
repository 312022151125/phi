package main

import (
	"fmt"
	"os"

	"github.com/pulseaiclub/xui"
	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/app"
	"github.com/pulseaiclub/phi/internal/config"
	"github.com/pulseaiclub/phi/internal/tui"
)

func main() {
	config, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
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
	m := tui.NewEditor(vx, th, cwd, config.Name, config.SkillPath)

	app := app.NewApp(vx)
	app.Anim = true
	m.App = app
	if err := app.Run(m); err != nil {
		panic(err)
	}
}
