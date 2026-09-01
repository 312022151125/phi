package controller

import (
	"time"

	"github.com/pulseaiclub/phi/internal/components/toast"
	"github.com/pulseaiclub/phi/internal/job"
	"github.com/pulseaiclub/phi/internal/permission"
	"github.com/pulseaiclub/phi/internal/session"
)

// Msg is a UI-thread message. Producers send; Editor.Update applies.
// Share memory by communicating — not the other way around.
type Msg interface {
	isMsg()
}

// SubmitMsg asks the UI to accept a user prompt.
type SubmitMsg struct{ Text string }

func (SubmitMsg) isMsg() {}

// CancelStreamMsg aborts the in-flight agent stream.
type CancelStreamMsg struct{}

func (CancelStreamMsg) isMsg() {}

// SessionEventMsg carries a session model event from the agent pipeline.
type SessionEventMsg struct{ Event session.Event }

func (SessionEventMsg) isMsg() {}

// SetActivityMsg sets footer/stream activity status.
type SetActivityMsg struct{ Activity Activity }

func (SetActivityMsg) isMsg() {}

// RedrawMsg only asks for a frame (e.g. delayed activity clear).
type RedrawMsg struct{}

func (RedrawMsg) isMsg() {}

// ToastMsg asks the Editor to show a transient overlay notification.
// Producers Publish this instead of holding a toast callback.
type ToastMsg struct {
	Message  string
	Kind     toast.ToastKind
	Duration time.Duration
}

func (ToastMsg) isMsg() {}

// ClearIfActivityMsg sets Idle only when current activity still matches If.
// Used for delayed "Stopped" → Idle without clobbering a newer state.
type ClearIfActivityMsg struct{ If Activity }

func (ClearIfActivityMsg) isMsg() {}

// MentionResultsMsg delivers async @-file search results to the UI goroutine.
type MentionResultsMsg struct {
	Gen   int
	Query string
	Paths []string
	// Truncated reports that more matches exist than Paths holds, so the
	// picker can say the list is partial instead of looking complete.
	Truncated bool
	ErrText   string
}

func (MentionResultsMsg) isMsg() {}

// PermissionAskMsg asks the UI to confirm a gated tool call.
// Reply must be buffered(1); the UI sends AskReply once.
type PermissionAskMsg struct {
	Request permission.Request
	Reason  string
	Reply   chan AskReply
}

func (PermissionAskMsg) isMsg() {}

// PermissionDismissMsg clears a pending permission overlay (timeout/cancel).
type PermissionDismissMsg struct{}

func (PermissionDismissMsg) isMsg() {}

// ContinueAskMsg asks the UI whether to grant another max-rounds budget.
// Reply must be buffered(1); the UI sends ContinueReply once.
type ContinueAskMsg struct {
	MaxRounds int
	Reply     chan ContinueReply
}

func (ContinueAskMsg) isMsg() {}

// ContinueDismissMsg clears a pending continue overlay (timeout/cancel).
type ContinueDismissMsg struct{}

func (ContinueDismissMsg) isMsg() {}

// UpdateAvailableMsg delivers a startup version-check result to the UI.
type UpdateAvailableMsg struct {
	Latest  string
	Current string
}

func (UpdateAvailableMsg) isMsg() {}

// ExtCommandResultMsg delivers the result of an extension slash command.
type ExtCommandResultMsg struct {
	Gen       uint64
	Submit    string
	Toast     string
	Status    string
	StatusSet bool
	Err       string
}

func (ExtCommandResultMsg) isMsg() {}

// ExtSessionEffectsMsg applies toast/status from session lifecycle extensions.
type ExtSessionEffectsMsg struct {
	Toast     string
	Status    string
	StatusSet bool
}

func (ExtSessionEffectsMsg) isMsg() {}

// ExtConfirmMsg asks the UI to show a yes/no dialog for an extension.
// Reply must be buffered(1).
type ExtConfirmMsg struct {
	Title   string
	Message string
	Yes     string
	No      string
	Danger  bool
	Reply   chan ExtConfirmReply
}

func (ExtConfirmMsg) isMsg() {}

// ExtConfirmDismissMsg clears a pending extension confirm (timeout/cancel).
type ExtConfirmDismissMsg struct{}

func (ExtConfirmDismissMsg) isMsg() {}

// JobProgressMsg carries a live sub-agent tool update for the nested tree UI.
type JobProgressMsg struct {
	Progress job.Progress
}

func (JobProgressMsg) isMsg() {}

// BranchLabelMsg refreshes the path label's git branch after an external
// checkout (e.g. from another terminal or editor).
type BranchLabelMsg struct {
	Text string
}

func (BranchLabelMsg) isMsg() {}
