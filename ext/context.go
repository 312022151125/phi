package ext

// Context is passed to event handlers and command handlers.
type Context struct {
	Cwd       string
	SessionID string
	HasUI     bool
	UI        UI
}

// ConfirmRequest describes a modal yes/no dialog.
type ConfirmRequest struct {
	Title   string
	Message string
	Yes     string // default "Yes"
	No      string // default "No"
	Danger  bool   // style Yes as destructive
}

// ConfirmReply is the user's choice.
type ConfirmReply struct {
	OK bool
}

// Pane is a non-blocking extension surface (multi-line body + optional actions).
type Pane struct {
	ID      string // empty → "default"
	Title   string
	Body    string
	Format  string // "text" (default) | "markdown"
	Actions []PaneAction
}

// PaneAction is a button on a pane. Click delivers OnPaneAction to the extension.
type PaneAction struct {
	ID    string
	Label string
	Kind  string // "" | "primary" | "danger"
}

// UI is the interactive surface available to extensions.
// Headless/run mode may provide a no-op or deny-by-default Confirm.
type UI interface {
	Notify(message, kind string) // kind: info | warning | error
	Confirm(title, message string) bool
	ConfirmOpts(ConfirmRequest) ConfirmReply
	SetStatus(key, text string) // empty text clears
	ShowPane(Pane)
	UpdatePane(id, body string)
	ClosePane(id string)
}
