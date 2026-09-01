package controller

// AskReply is the user's response for a gated tool confirmation.
type AskReply struct {
	Approved        bool
	Feedback        string
	AllowSession    bool // Allow All for This Session
	AllowPersistent bool // Allow All for Every Session
}

// ExtConfirmReply is the user's response for an extension Confirm dialog.
type ExtConfirmReply struct {
	OK bool
}

// ContinueReply is the user's response when the tool-round budget is exhausted.
type ContinueReply struct {
	Continue bool
}
