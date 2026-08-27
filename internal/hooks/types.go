package hooks

import "encoding/json"

// Event is the tool-loop payload (alias for ToolEvent).
type Event = ToolEvent

// ToolEvent is the runtime payload for PreToolUse / PostToolUse / PostToolUseFailure.
type ToolEvent struct {
	SessionID string          `json:"session_id"`
	Cwd       string          `json:"cwd"`
	Tool      string          `json:"tool"`
	ToolUseID string          `json:"tool_use_id"`
	Input     json.RawMessage `json:"input"`
	Output    string          `json:"output,omitempty"`
	Err       string          `json:"error,omitempty"`
}

// SessionEvent is the runtime payload for session lifecycle hooks.
type SessionEvent struct {
	SessionID         string
	Cwd               string
	Reason            string // startup | new | resume | quit
	PreviousSessionID string
	TargetSessionID   string
	MessageID         string
	Usage             SessionUsage
}

type SessionUsage struct {
	PromptTokens     int
	CompletionTokens int
	CachedTokens     int
	TotalTokens      int
}

// CommandEvent is the payload for slash-command hooks.
type CommandEvent struct {
	SessionID string
	Cwd       string
	Args      []string
}

// CommandList is a palette page of selectable rows.
type CommandList struct {
	Title string            `json:"title"`
	Items []CommandListItem `json:"items"`
}

// CommandListItem is one row in a CommandList.
type CommandListItem struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Submit string `json:"submit"`
}

// CommandResult is returned from a slash-command hook.
type CommandResult struct {
	Submit    string
	Toast     string
	Status    string
	StatusSet bool
	List      *CommandList
}

// CommandEntry is a registered slash command from a Command hook.
type CommandEntry struct {
	Name string
}

// PreOutcome is the aggregated PreToolUse decision for the executor.
type PreOutcome struct {
	Input   json.RawMessage
	Denied  bool
	Reason  string
	Context string
}

// PostOutcome is the aggregated PostToolUse decision for the executor.
type PostOutcome struct {
	Context string
	Stop    bool
	Reason  string
	Output  string
}

// SessionOutcome aggregates session lifecycle hook UI signals.
type SessionOutcome struct {
	Denied    bool
	Reason    string
	Toast     string
	Status    string
	StatusSet bool
}
