package ext

// Lifecycle event names for coding-agent extensions.
const (
	EventToolCall            = "tool_call"
	EventToolResult          = "tool_result"
	EventToolExecutionStart  = "tool_execution_start"
	EventToolExecutionEnd    = "tool_execution_end"
	EventSessionStart        = "session_start"
	EventSessionShutdown     = "session_shutdown"
	EventSessionBeforeSwitch = "session_before_switch"
	EventBeforeAgentStart    = "before_agent_start"
	EventAgentStart          = "agent_start"
	EventAgentEnd            = "agent_end"
	EventTurnStart           = "turn_start"
	EventTurnEnd             = "turn_end"
)
