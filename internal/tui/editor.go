package tui

import (
	"strings"
	"time"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/app"
	"github.com/pulseaiclub/phi/internal/components/chat"
	"github.com/pulseaiclub/phi/internal/components/layout"
	"github.com/pulseaiclub/phi/internal/components/palette"
	"github.com/pulseaiclub/phi/internal/components/splash"
	"github.com/pulseaiclub/phi/internal/components/status"
	"github.com/pulseaiclub/phi/internal/components/toast"
	"github.com/pulseaiclub/phi/internal/components/transcript"
	"github.com/pulseaiclub/phi/internal/session"
	"github.com/pulseaiclub/xui"
)

// Editor is the TUI root widget: layout composition and the UI-goroutine
// message loop. Cross-component work goes through Bus — producers Publish,
// Draw drains and Update applies. Agent lifecycle lives in Controller;
// session→widget projection lives in Mapper; activity status in ActivityHandler.
type Editor struct {
	vx    *xui.XUI
	App   *app.App
	theme components.Theme
	bus   *Bus

	list      transcript.MessageList
	Chat      chat.ChatInput
	palette   palette.CommandPalette
	toast     toast.Toast
	spin      *status.Spinner
	welcome   splash.Screen
	activity  *ActivityHandler
	startedAt time.Time
	tick      int

	listH        int
	lastListSurf components.Surface
	sel          textSel

	// Session model. Mutations happen only on the UI goroutine via Update.
	snap session.Snapshot

	mapper  *Mapper
	ctrl    *Controller
	listIDs []string // parallels list.Entries (item ids)
}

func newChatInput(theme components.Theme, model string, cwd string) chat.ChatInput {
	return chat.ChatInput{
		MinBodyRows:    3,
		MaxBodyRows:    8,
		UseBlockCursor: false, // terminal cursor only; reverse cells ghost on CJK delete
		PaddingX:       1,
		Theme:          theme,
		BorderStyle:    theme.Border,
		TextStyle:      theme.Foreground,
		CursorStyle:    xui.Style{Reverse: true},
		// TopLeftLabel: layout.BorderLabel{
		// 	Text:  "4% of 300k",
		// 	Style: theme.Muted,
		// },
		TopRightLabel: layout.BorderLabel{
			Text:  model,
			Style: theme.Success,
		},
		BottomRightLabel: layout.BorderLabel{
			Text:  shortPath(cwd),
			Style: theme.Muted,
		},
	}
}

func NewEditor(vx *xui.XUI, theme components.Theme, cwd string, model string, skillPath string) *Editor {
	editor := &Editor{
		vx:        vx,
		theme:     theme,
		Chat:      newChatInput(theme, model, cwd),
		spin:      status.NewSpinner(theme.ToolName),
		startedAt: time.Now(),
		welcome: splash.Screen{
			Sphere: &splash.Sphere{Fast: true},
			Theme:  theme,
			Brand:  "Phi",
		},
		palette: palette.CommandPalette{
			Theme: theme,
		},
		toast: toast.Toast{Theme: theme},
		list: transcript.MessageList{
			Theme:    theme,
			Selected: -1,
		},
	}
	editor.activity = NewActivityHandler(editor.spin)
	editor.bus = NewBus(editor.requestRedraw)
	editor.mapper = NewMapper(theme, editor.spin, func() {
		editor.list.InvalidateHeights()
	})
	editor.ctrl = NewController(editor.bus)
	editor.palette.FocusReturn = &editor.Chat
	editor.Chat.OnSubmit = func(text string) {
		// OnSubmit runs on the UI goroutine — publish and apply immediately
		// so the composer clears before the next frame.
		editor.Publish(SubmitMsg{Text: text})
		editor.drainBus()
	}
	editor.Chat.OnChange = func(string) {
		// CJK paste/delete can desync tty wide-glyph columns vs our damage
		// grid; force a full redraw so ghost cells cannot stick around.
		if editor.vx != nil {
			editor.vx.QueueRefresh()
		}
	}
	addSkill := func(name string) {
		editor.Chat.AddPendingSkill(name)
		if editor.vx != nil {
			editor.vx.QueueRefresh()
		}
	}
	editor.palette.Commands = append(
		PaletteCommands(func(msg string) {
			editor.list.StickToBottom()
		}),
		SkillsCommand(skillPath, addSkill),
		palette.PaletteCommand{
			ID:       "clipboard-copy-last",
			Noun:     "clipboard",
			Verb:     "copy last message",
			Keywords: []string{"yank", "selection"},
			Shortcut: "Ctrl+Shift+C",
			Run: func() {
				text := editor.list.LastCopyText()
				editor.copyBlock(text)
			},
		},
	)
	return editor
}

// Publish sends a message onto the bus from any goroutine / widget callback.
func (editor *Editor) Publish(m Msg) {
	if editor.bus != nil {
		editor.bus.Publish(m)
	}
}

// Update applies one message on the UI goroutine. Returns whether a redraw is useful.
func (editor *Editor) Update(m Msg) {
	switch msg := m.(type) {
	case SubmitMsg:
		editor.handleSubmit(msg.Text)
	case CancelStreamMsg:
		editor.handleCancel()
	case SessionEventMsg:
		editor.applySessionEvent(msg.Event)
	case SetActivityMsg:
		editor.activity.Apply(msg.Activity)
	case ClearIfActivityMsg:
		if editor.activity.Current == msg.If {
			editor.activity.Apply(ActivityIdle)
		}
	case RedrawMsg:
		// no state change; drain already requested redraw
	}
}

func (editor *Editor) drainBus() {
	batch := editor.bus.Drain()
	if len(batch) == 0 {
		return
	}
	atBottom := editor.list.ScrollFromBottom == 0
	threadDirty := false
	for _, m := range batch {
		if _, ok := m.(SessionEventMsg); ok {
			threadDirty = true
		}
		editor.Update(m)
	}
	if threadDirty {
		editor.list.Entries, editor.listIDs = editor.mapper.Sync(editor.list.Entries, editor.listIDs, editor.snap)
		editor.list.InvalidateHeights()
		editor.activity.SyncFromSnap(editor.snap)
		if atBottom {
			editor.list.StickToBottom()
		}
	}
}

func (editor *Editor) applySessionEvent(ev session.Event) {
	editor.snap = session.Apply(editor.snap, ev)
}

func (editor *Editor) handleSubmit(text string) {
	text = strings.TrimSpace(text)
	pending := append([]string(nil), editor.Chat.PendingSkills...)
	if (text == "" && len(pending) == 0) || editor.isBusy() {
		return
	}

	editor.activity.Apply(ActivitySubmitting)
	display := text
	if display == "" && len(pending) > 0 {
		display = "Skills: " + strings.Join(pending, ", ")
	}
	editor.applySessionEvent(session.UserAppend{Text: display})
	editor.list.Entries, editor.listIDs = editor.mapper.Sync(editor.list.Entries, editor.listIDs, editor.snap)
	editor.list.InvalidateHeights()
	editor.list.StickToBottom()
	editor.activity.Apply(ActivityWaiting)

	editor.Chat.Value = ""
	editor.Chat.Cursor = 0
	editor.Chat.ClearPendingSkills()

	editor.ctrl.StartPrompt(text, pending)
}

func (editor *Editor) handleCancel() {
	editor.ctrl.Cancel()
	editor.applySessionEvent(session.CancelStreaming{})
	editor.list.Entries, editor.listIDs = editor.mapper.Sync(editor.list.Entries, editor.listIDs, editor.snap)
	editor.list.InvalidateHeights()
	editor.activity.Apply(ActivityCancelled)
	time.AfterFunc(1200*time.Millisecond, func() {
		editor.Publish(ClearIfActivityMsg{If: ActivityCancelled})
	})
}

func (editor *Editor) Handle(ctx *components.EventContext, ev xui.Event) {
	switch e := ev.(type) {
	case xui.FocusEvent:
		if editor.palette.Open {
			ctx.RequestFocus(&editor.palette)
		} else {
			ctx.RequestFocus(&editor.Chat)
		}
	case xui.KeyEvent:
		if e.CtrlC() {
			ctx.Quit = true
			return
		}
		if editor.handleCopyKey(ctx, e) {
			return
		}
		if e.Press && e.Code == xui.KeyEscape {
			if editor.isBusy() {
				editor.Publish(CancelStreamMsg{})
				editor.drainBus()
				ctx.ConsumeAndRedraw()
				return
			}
			if editor.sel.active {
				editor.sel.clear()
				ctx.ConsumeAndRedraw()
				return
			}
		}
		if e.Press && e.Mods.Has(xui.ModCtrl) && e.Code == xui.KeyRune &&
			(e.Rune == 'k' || e.Rune == 'K') {
			if editor.palette.Open {
				editor.palette.Hide()
				ctx.RequestFocus(&editor.Chat)
			} else {
				editor.palette.Show()
				ctx.RequestFocus(&editor.palette)
			}
			ctx.ConsumeAndRedraw()
			return
		}
		if editor.palette.Open {
			editor.palette.Handle(ctx, e)
			if !editor.palette.Open {
				ctx.RequestFocus(&editor.Chat)
			}
			return
		}
		if e.Code == xui.KeyPageUp || e.Code == xui.KeyPageDown {
			editor.list.Handle(ctx, e)
			return
		}
		// Fallback: if focus was left on a transcript widget, still type into
		// the composer (keys bubble here when the focused widget ignores them).
		editor.Chat.Handle(ctx, e)
	case xui.MouseEvent:
		if editor.palette.Open {
			editor.palette.Handle(ctx, e)
			return
		}
		editor.handleListMouse(ctx, e)
	case xui.PasteEvent:
		if editor.palette.Open {
			editor.palette.Handle(ctx, e)
			return
		}
		editor.Chat.Handle(ctx, e)
	}
}

func (editor *Editor) Draw(ctx components.DrawContext) components.Surface {
	editor.drainBus()

	editor.tick++
	if editor.activity.ShowSpinner() && editor.tick%4 == 0 {
		editor.spin.Tick()
	}
	if editor.welcome.Sphere != nil {
		editor.welcome.Sphere.Time = time.Since(editor.startedAt).Seconds()
	}
	_ = editor.toast.Visible()

	maxSize := ctx.Max
	root := components.Surface{Size: maxSize, Widget: editor}

	footerH := 1
	chatH := editor.Chat.PreferredHeight(maxSize.Width, ctx.Method)
	minChatH := 5
	if len(editor.Chat.PendingSkills) > 0 {
		minChatH++
	}
	if chatH < minChatH {
		chatH = minChatH
	}
	maxChatH := maxSize.Height - footerH - 3
	if maxChatH < minChatH {
		maxChatH = minChatH
	}
	if chatH > maxChatH {
		chatH = maxChatH
	}
	listH := maxSize.Height - chatH - footerH
	if listH < 3 {
		listH = 3
		chatH = maxSize.Height - listH - footerH
		if chatH < minChatH {
			chatH = minChatH
		}
	}
	editor.listH = listH

	var listSurf components.Surface
	if len(editor.list.Entries) == 0 {
		listSurf = editor.welcome.Draw(ctx.WithConstraints(components.Size{}, components.Size{Width: maxSize.Width, Height: listH}))
	} else {
		listSurf = editor.list.Draw(ctx.WithConstraints(components.Size{}, components.Size{Width: maxSize.Width, Height: listH}))
	}
	if editor.sel.active {
		hl := editor.theme.SelectionBg
		hl.Fg = editor.theme.SelectionFg.Fg
		ax, ay, ex, ey := editor.viewSel()
		components.ApplySelectionHighlight(&listSurf, ax, ay, ex, ey, hl)
	}
	editor.lastListSurf = listSurf

	chatSurf := editor.Chat.Draw(ctx.WithConstraints(components.Size{}, components.Size{Width: maxSize.Width, Height: chatH}))
	footer := editor.drawFooter(ctx, maxSize.Width)

	root.Children = []components.SubSurface{
		{Origin: components.Point{X: 0, Y: 0}, Surface: listSurf},
		{Origin: components.Point{X: 0, Y: listH}, Surface: chatSurf, Z: 1},
		{Origin: components.Point{X: 0, Y: maxSize.Height - footerH}, Surface: footer, Z: 2},
	}
	if editor.palette.Open {
		pal := editor.palette.Draw(ctx)
		root.Children = append(root.Children, components.SubSurface{
			Origin:  components.Point{X: 0, Y: 0},
			Surface: pal,
			Z:       20,
		})
	}
	if editor.toast.Visible() {
		toastSurf := editor.toast.Draw(ctx)
		root.Children = append(root.Children, components.SubSurface{
			Origin:  components.Point{X: 0, Y: 0},
			Surface: toastSurf,
			Z:       40,
		})
	}
	return root
}

func (editor *Editor) isBusy() bool {
	return session.IsStreaming(editor.snap)
}

func (editor *Editor) requestRedraw() {
	if editor.App != nil {
		editor.App.RequestRedraw()
	}
}

// SubmitPrompt is kept for callers; it publishes onto the bus.
func (editor *Editor) SubmitPrompt(text string) {
	editor.Publish(SubmitMsg{Text: text})
}
