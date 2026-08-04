package tui

import (
	"strings"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/palette"
	"github.com/pulseaiclub/phi/internal/llm/skills"
)

// PaletteCommands returns sample commands for the tui demo.
func PaletteCommands(onRun func(msg string)) []palette.PaletteCommand {
	run := func(msg string) func() {
		return func() {
			if onRun != nil {
				onRun(msg)
			}
		}
	}

	models := []palette.PaletteCommand{
		{ID: "model-deepseek-v4-pro", Verb: "deepseek", Run: run("model: Model")},
	}
	return []palette.PaletteCommand{
		{
			ID:           "settings-model",
			Noun:         "settings",
			Verb:         "model",
			Keywords:     []string{"model"},
			SubmenuTitle: "Select Model",
			Submenu:      models,
		},
	}
}

// ThemeCommand returns a settings → theme submenu listing builtin palettes.
func ThemeCommand(apply func(name string)) palette.PaletteCommand {
	names := components.ThemeNames()
	submenu := make([]palette.PaletteCommand, 0, len(names))
	for _, name := range names {
		n := name
		submenu = append(submenu, palette.PaletteCommand{
			ID:       "theme-" + strings.ToLower(n),
			Verb:     n + " (builtin)",
			Keywords: []string{n, "theme", "color"},
			Run: func() {
				if apply != nil {
					apply(n)
				}
			},
		})
	}
	return palette.PaletteCommand{
		ID:           "settings-theme",
		Noun:         "settings",
		Verb:         "theme",
		Keywords:     []string{"theme", "color", "appearance", "dark", "darcula", "pink"},
		SubmenuTitle: "Select Theme",
		Submenu:      submenu,
	}
}

// SkillsCommand returns a top-level "skills" palette entry whose submenu lists
// every skill discovered under skillPath. Selecting one adds it as a pending skill.
func SkillsCommand(skillPath string, add func(name string)) palette.PaletteCommand {
	submenu := skillSubcommands(skillPath, add)
	return palette.PaletteCommand{
		ID:           "skills",
		Noun:         "skills",
		Verb:         "invoke",
		Keywords:     []string{"skill", "use skill", "load skill", "pending"},
		SubmenuTitle: "Select skill",
		Submenu:      submenu,
	}
}

func skillSubcommands(skillPath string, add func(name string)) []palette.PaletteCommand {
	list, err := skills.LoadSkills(skillPath)
	if err != nil || len(list) == 0 {
		return []palette.PaletteCommand{{
			ID:       "skills-empty",
			Verb:     "No skills found",
			Disabled: true,
		}}
	}

	out := make([]palette.PaletteCommand, 0, len(list))
	for _, s := range list {
		name := s.Name
		if strings.TrimSpace(name) == "" {
			name = strings.TrimSpace(s.Path)
		}
		desc := s.Description
		out = append(out, palette.PaletteCommand{
			ID:       "skill-" + name,
			Verb:     name,
			Keywords: []string{desc, "skill"},
			Run: func() {
				if add != nil {
					add(name)
				}
			},
		})
	}
	return out
}
