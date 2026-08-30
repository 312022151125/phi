package editor

import (
	"time"

	"github.com/pulseaiclub/phi/internal/components/palette"
	"github.com/pulseaiclub/phi/internal/components/toast"
	"github.com/pulseaiclub/phi/internal/tui/commands"
	"github.com/pulseaiclub/phi/internal/tui/composer"
	"github.com/pulseaiclub/phi/internal/tui/controller"
	"github.com/pulseaiclub/phi/internal/tui/submit"
	"github.com/pulseaiclub/phi/internal/tui/transcript"
)

type commandBridge struct {
	bus        *controller.Bus
	composer   *composer.ComposerPane
	transcript *transcript.TranscriptPane
	ctrl       *controller.EngineController
	submitter  *submit.Submitter
	sessions   *commands.SessionCommands

	reloadExtensions func()
	listExtensions   func() []palette.PaletteCommand
	setModel         func(string)
	applyTheme       func(string)
	setPermissions   func(bool)
	setAgents        func(bool)
	addSkill         func(string)
	copyLastMessage  func()

	modelNames []string
	skillPath  string
}

func newCommandBridge(
	bus *controller.Bus,
	composer *composer.ComposerPane,
	transcript *transcript.TranscriptPane,
	ctrl *controller.EngineController,
	submitter *submit.Submitter,
	sessions *commands.SessionCommands,
	modelNames []string,
	skillPath string,
	reloadExtensions func(),
	listExtensions func() []palette.PaletteCommand,
	setModel func(string),
	applyTheme func(string),
	setPermissions func(bool),
	setAgents func(bool),
	addSkill func(string),
	copyLastMessage func(),
) *commandBridge {
	return &commandBridge{
		bus:              bus,
		composer:         composer,
		transcript:       transcript,
		ctrl:             ctrl,
		submitter:        submitter,
		sessions:         sessions,
		reloadExtensions: reloadExtensions,
		listExtensions:   listExtensions,
		setModel:         setModel,
		applyTheme:       applyTheme,
		setPermissions:   setPermissions,
		setAgents:        setAgents,
		addSkill:         addSkill,
		copyLastMessage:  copyLastMessage,
		modelNames:       append([]string(nil), modelNames...),
		skillPath:        skillPath,
	}
}

func (b *commandBridge) context() commands.CommandContext {
	if b == nil {
		return commands.CommandContext{}
	}
	return commands.CommandContext{
		Bus: b.bus,
		PushSubmenu: func(title string, cmds []palette.PaletteCommand) {
			b.composer.PushPalette(title, cmds)
		},
		ShowSessions:  b.sessions.Show,
		ResumeSession: b.sessions.Resume,
		ClearSession: func() {
			if b.submitter != nil && b.submitter.StreamActive() {
				b.bus.Publish(controller.ToastMsg{
					Message:  "Cannot clear while a reply or command is running",
					Kind:     toast.ToastWarning,
					Duration: 3 * time.Second,
				})
				return
			}
			b.sessions.Clear()
		},
		SetModel:         b.setModel,
		ApplyTheme:       b.applyTheme,
		SetPermissions:   b.setPermissions,
		SetAgents:        b.setAgents,
		ReloadExtensions: b.reloadExtensions,
		ListExtensions:   b.listExtensions,
		AddSkill:         b.addSkill,
		CopyLastMessage:  b.copyLastMessage,
		ModelNames:       b.modelNames,
		SkillPath:        b.skillPath,
	}
}
