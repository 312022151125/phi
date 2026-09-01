package extension

import "github.com/pulseaiclub/phi/ext"

// BusUI publishes UI effects onto host callbacks.
type BusUI struct {
	NotifyFn     func(message, kind string)
	ConfirmFn    func(ext.ConfirmRequest) ext.ConfirmReply
	SetStatusFn  func(key, text string)
	ShowPaneFn   func(ext.Pane)
	UpdatePaneFn func(id, body string)
	ClosePaneFn  func(id string)
}

func (u BusUI) Notify(message, kind string) {
	if u.NotifyFn != nil {
		u.NotifyFn(message, kind)
	}
}

func (u BusUI) Confirm(title, message string) bool {
	return u.ConfirmOpts(ext.ConfirmRequest{Title: title, Message: message}).OK
}

func (u BusUI) ConfirmOpts(req ext.ConfirmRequest) ext.ConfirmReply {
	if u.ConfirmFn != nil {
		return u.ConfirmFn(req)
	}
	return ext.ConfirmReply{}
}

func (u BusUI) SetStatus(key, text string) {
	if u.SetStatusFn != nil {
		u.SetStatusFn(key, text)
	}
}

func (u BusUI) ShowPane(p ext.Pane) {
	if u.ShowPaneFn != nil {
		u.ShowPaneFn(p)
	}
}

func (u BusUI) UpdatePane(id, body string) {
	if u.UpdatePaneFn != nil {
		u.UpdatePaneFn(id, body)
	}
}

func (u BusUI) ClosePane(id string) {
	if u.ClosePaneFn != nil {
		u.ClosePaneFn(id)
	}
}

var _ ext.UI = BusUI{}
