package ext

// Context is passed to event handlers and command handlers.
type Context struct {
	Cwd       string
	SessionID string
	HasUI     bool
	UI        UI
}

// UI is the interactive surface available to extensions.
// Headless/run mode may provide a no-op or deny-by-default Confirm.
type UI interface {
	Notify(message, kind string) // kind: info | warning | error
	Confirm(title, message string) bool
	SetStatus(key, text string) // empty text clears
}
