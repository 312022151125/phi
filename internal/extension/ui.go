package extension

import "github.com/pulseaiclub/phi/ext"

// BusUI publishes Notify as toast-like callbacks. Confirm always returns false
// until a real dialog is wired (safe default for policy gates that need explicit approval).
type BusUI struct {
	NotifyFn    func(message, kind string)
	ConfirmFn   func(title, message string) bool
	SetStatusFn func(key, text string)
}

func (u BusUI) Notify(message, kind string) {
	if u.NotifyFn != nil {
		u.NotifyFn(message, kind)
	}
}

func (u BusUI) Confirm(title, message string) bool {
	if u.ConfirmFn != nil {
		return u.ConfirmFn(title, message)
	}
	return false
}

func (u BusUI) SetStatus(key, text string) {
	if u.SetStatusFn != nil {
		u.SetStatusFn(key, text)
	}
}

// Ensure BusUI implements ext.UI.
var _ ext.UI = BusUI{}
