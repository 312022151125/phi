package commands

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pulseaiclub/phi/internal/components/palette"
	"github.com/pulseaiclub/phi/internal/components/toast"
	"github.com/pulseaiclub/phi/internal/debuglog"
	"github.com/pulseaiclub/phi/internal/hooks"
	"github.com/pulseaiclub/phi/internal/tui/controller"
)

type hookComposer interface {
	SetPaletteCommands([]palette.PaletteCommand)
	PushPalette(title string, cmds []palette.PaletteCommand)
}

type hookFooter interface {
	SetHookStatus(status string)
}

type hookSubmitter interface {
	IsBusy() bool
	Submit(text string)
}

// HookCommands owns slash commands registered from Command hooks.
type HookCommands struct {
	Registry   *CommandRegistry
	Ctrl       *controller.EngineController
	CWD        string
	Composer   hookComposer
	Footer     hookFooter
	Submitter  hookSubmitter
	Publish    func(controller.Msg)
	CommandCtx func() CommandContext

	gen     atomic.Uint64
	running atomic.Bool
}

func (h *HookCommands) showToast(msg string, kind toast.ToastKind) {
	if h == nil || h.Publish == nil {
		return
	}
	h.Publish(controller.ToastMsg{Message: msg, Kind: kind, Duration: 3 * time.Second})
}

// Sync replaces hook-sourced slash commands from the current hooks.Manager.
func (h *HookCommands) Sync() {
	if h == nil || h.Registry == nil {
		return
	}
	h.gen.Add(1)
	h.Registry.clearHookCommands()
	if h.Ctrl != nil {
		for _, entry := range h.Ctrl.Hooks().CommandEntries() {
			name := entry.Name
			if !h.Registry.registerHook(h.slashCommand(name)) {
				debuglog.Logf("hooks: command %q skipped (name already registered)", name)
			}
		}
	}
	ctx := CommandContext{}
	if h.CommandCtx != nil {
		ctx = h.CommandCtx()
	}
	h.Composer.SetPaletteCommands(h.Registry.BuildPalette(ctx))
}

func (h *HookCommands) slashCommand(name string) Command {
	return Command{
		Name:        name,
		Description: "hook command",
		Slash:       true,
		Insert:      "/" + name,
		Run: func(ctx CommandContext) error {
			if h.running.Load() {
				ctx.toast("A hook command is already running", toast.ToastWarning, 3*time.Second)
				return nil
			}
			args := append([]string(nil), ctx.Args...)
			go h.run(name, args)
			return nil
		},
	}
}

func (h *HookCommands) run(name string, args []string) {
	if h == nil {
		return
	}
	if !h.running.CompareAndSwap(false, true) {
		h.Publish(controller.HookCommandResultMsg{
			Gen: h.gen.Load(),
			Err: "A hook command is already running",
		})
		return
	}
	defer h.running.Store(false)

	gen := h.gen.Load()
	if h.Ctrl == nil {
		h.Publish(controller.HookCommandResultMsg{Gen: gen, Err: "hooks are not loaded"})
		return
	}
	mgr := h.Ctrl.Hooks()
	if mgr == nil {
		h.Publish(controller.HookCommandResultMsg{Gen: gen, Err: "hooks are not loaded"})
		return
	}
	res, err := mgr.RunCommand(context.Background(), name, hooks.CommandEvent{
		SessionID: h.Ctrl.SessionID(),
		Cwd:       h.CWD,
		Args:      args,
	})
	if gen != h.gen.Load() {
		return
	}
	if err != nil {
		h.Publish(controller.HookCommandResultMsg{Gen: gen, Err: err.Error()})
		return
	}
	h.Publish(controller.HookCommandResultMsg{
		Gen:       gen,
		Submit:    res.Submit,
		Toast:     res.Toast,
		Status:    res.Status,
		StatusSet: res.StatusSet,
		List:      res.List,
	})
}

// Apply delivers a finished hook command onto the UI goroutine.
func (h *HookCommands) Apply(msg controller.HookCommandResultMsg) {
	if h == nil || msg.Gen != h.gen.Load() {
		return
	}
	if msg.Err != "" {
		h.showToast(msg.Err, toast.ToastError)
		return
	}
	h.applyIntents(msg)
}

func (h *HookCommands) applyIntents(msg controller.HookCommandResultMsg) {
	if msg.StatusSet {
		h.Footer.SetHookStatus(msg.Status)
	}
	if msg.Toast != "" {
		h.showToast(msg.Toast, toast.ToastSuccess)
	}
	if msg.List != nil && len(msg.List.Items) > 0 {
		h.pushList(*msg.List)
		return
	}
	if msg.Submit != "" {
		if h.Submitter.IsBusy() {
			h.showToast("Cannot submit hook command while a reply is running", toast.ToastWarning)
			return
		}
		h.Submitter.Submit(msg.Submit)
	}
}

func (h *HookCommands) pushList(list hooks.CommandList) {
	title := list.Title
	if title == "" {
		title = "Hook"
	}
	cmds := make([]palette.PaletteCommand, 0, len(list.Items))
	for _, item := range list.Items {
		label := item.Label
		if label == "" {
			label = item.Submit
		}
		if label == "" {
			continue
		}
		cmds = append(cmds, palette.PaletteCommand{
			Verb:     label,
			Keywords: keywordsForDetail(item.Detail),
			Run: func() {
				if item.Submit == "" {
					return
				}
				if h.Submitter.IsBusy() {
					h.showToast("Cannot submit while a reply is running", toast.ToastWarning)
					return
				}
				h.Submitter.Submit(item.Submit)
			},
		})
	}
	if len(cmds) == 0 {
		h.showToast("Hook list had no usable items", toast.ToastWarning)
		return
	}
	h.Composer.PushPalette(title, cmds)
}

func keywordsForDetail(detail string) []string {
	if detail == "" {
		return nil
	}
	return []string{detail}
}
