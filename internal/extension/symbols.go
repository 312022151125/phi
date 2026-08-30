package extension

import (
	"reflect"

	"github.com/pulseaiclub/yaegi/interp"

	"github.com/pulseaiclub/phi/ext"
)

// extSymbols exports github.com/pulseaiclub/phi/ext for interpreted extensions.
func extSymbols() interp.Exports {
	return interp.Exports{
		"github.com/pulseaiclub/phi/ext/ext": map[string]reflect.Value{
			// types
			"API":                       reflect.ValueOf((*ext.API)(nil)),
			"Context":                   reflect.ValueOf((*ext.Context)(nil)),
			"ToolCallEvent":             reflect.ValueOf((*ext.ToolCallEvent)(nil)),
			"ToolCallResult":            reflect.ValueOf((*ext.ToolCallResult)(nil)),
			"ToolResultEvent":           reflect.ValueOf((*ext.ToolResultEvent)(nil)),
			"ToolResultResult":          reflect.ValueOf((*ext.ToolResultResult)(nil)),
			"ToolExecutionStartEvent":   reflect.ValueOf((*ext.ToolExecutionStartEvent)(nil)),
			"ToolExecutionEndEvent":     reflect.ValueOf((*ext.ToolExecutionEndEvent)(nil)),
			"SessionStartEvent":         reflect.ValueOf((*ext.SessionStartEvent)(nil)),
			"SessionShutdownEvent":      reflect.ValueOf((*ext.SessionShutdownEvent)(nil)),
			"SessionBeforeSwitchEvent":  reflect.ValueOf((*ext.SessionBeforeSwitchEvent)(nil)),
			"SessionBeforeSwitchResult": reflect.ValueOf((*ext.SessionBeforeSwitchResult)(nil)),
			"BeforeAgentStartEvent":     reflect.ValueOf((*ext.BeforeAgentStartEvent)(nil)),
			"BeforeAgentStartResult":    reflect.ValueOf((*ext.BeforeAgentStartResult)(nil)),
			"AgentStartEvent":           reflect.ValueOf((*ext.AgentStartEvent)(nil)),
			"AgentEndEvent":             reflect.ValueOf((*ext.AgentEndEvent)(nil)),
			"TurnStartEvent":            reflect.ValueOf((*ext.TurnStartEvent)(nil)),
			"TurnEndEvent":              reflect.ValueOf((*ext.TurnEndEvent)(nil)),
			"ToolDef":                   reflect.ValueOf((*ext.ToolDef)(nil)),
			"ToolResult":                reflect.ValueOf((*ext.ToolResult)(nil)),
			"ToolInfo":                  reflect.ValueOf((*ext.ToolInfo)(nil)),
			"CommandDef":                reflect.ValueOf((*ext.CommandDef)(nil)),
			"CommandEntry":              reflect.ValueOf((*ext.CommandEntry)(nil)),
			"ExecResult":                reflect.ValueOf((*ext.ExecResult)(nil)),
			"SessionEffects":            reflect.ValueOf((*ext.SessionEffects)(nil)),
			"HostOpts":                  reflect.ValueOf((*ext.HostOpts)(nil)),

			// constants
			"EventToolCall":            reflect.ValueOf(ext.EventToolCall),
			"EventToolResult":          reflect.ValueOf(ext.EventToolResult),
			"EventToolExecutionStart":  reflect.ValueOf(ext.EventToolExecutionStart),
			"EventToolExecutionEnd":    reflect.ValueOf(ext.EventToolExecutionEnd),
			"EventSessionStart":        reflect.ValueOf(ext.EventSessionStart),
			"EventSessionShutdown":     reflect.ValueOf(ext.EventSessionShutdown),
			"EventSessionBeforeSwitch": reflect.ValueOf(ext.EventSessionBeforeSwitch),
			"EventBeforeAgentStart":    reflect.ValueOf(ext.EventBeforeAgentStart),
			"EventAgentStart":          reflect.ValueOf(ext.EventAgentStart),
			"EventAgentEnd":            reflect.ValueOf(ext.EventAgentEnd),
			"EventTurnStart":           reflect.ValueOf(ext.EventTurnStart),
			"EventTurnEnd":             reflect.ValueOf(ext.EventTurnEnd),

			// constructors
			"NewAPI": reflect.ValueOf(ext.NewAPI),
		},
	}
}
