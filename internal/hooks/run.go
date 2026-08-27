package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultTimeout   = 5 * time.Second
	maxTimeout       = 60 * time.Second
	maxHookOutput    = 1 << 20 // 1 MiB
	exitBlocking     = 2
	pluginRootEnvKey = "PHI_PLUGIN_ROOT"
)

type binding struct {
	pluginID string
	dir      string
	event    HookEvent
	matcher  string
	cmd      CommandHook
}

type hookInput struct {
	SessionID         string          `json:"session_id"`
	Cwd               string          `json:"cwd"`
	HookEventName     string          `json:"hook_event_name"`
	ToolName          string          `json:"tool_name,omitempty"`
	ToolInput         json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID         string          `json:"tool_use_id,omitempty"`
	ToolResponse      string          `json:"tool_response,omitempty"`
	Error             string          `json:"error,omitempty"`
	PreviousSessionID string          `json:"previous_session_id,omitempty"`
	TargetSessionID   string          `json:"target_session_id,omitempty"`
	Source            string          `json:"source,omitempty"`
	Reason            string          `json:"reason,omitempty"`
	Command           string          `json:"command,omitempty"`
	Args              []string        `json:"args,omitempty"`
	MessageID         string          `json:"message_id,omitempty"`
	Usage             *hookUsage      `json:"usage,omitempty"`
}

type hookUsage struct {
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
	CachedTokens     int `json:"cached_tokens,omitempty"`
	TotalTokens      int `json:"total_tokens,omitempty"`
}

type syncHookOutput struct {
	Continue           *bool           `json:"continue"`
	StopReason         string          `json:"stopReason"`
	SystemMessage      string          `json:"systemMessage"`
	Decision           string          `json:"decision"`
	Reason             string          `json:"reason"`
	HookSpecificOutput json.RawMessage `json:"hookSpecificOutput"`
}

type preToolSpecific struct {
	HookEventName            string          `json:"hookEventName"`
	PermissionDecision       string          `json:"permissionDecision"`
	PermissionDecisionReason string          `json:"permissionDecisionReason"`
	UpdatedInput             json.RawMessage `json:"updatedInput"`
	AdditionalContext        string          `json:"additionalContext"`
}

type postToolSpecific struct {
	HookEventName        string `json:"hookEventName"`
	AdditionalContext    string `json:"additionalContext"`
	UpdatedMCPToolOutput string `json:"updatedMCPToolOutput"`
}

type sessionStartSpecific struct {
	HookEventName      string   `json:"hookEventName"`
	AdditionalContext  string   `json:"additionalContext"`
	InitialUserMessage string   `json:"initialUserMessage"`
	WatchPaths         []string `json:"watchPaths"`
}

func (b binding) run(ctx context.Context, in hookInput) (syncHookOutput, int, string, error) {
	stdout, code, err := b.spawn(ctx, in)
	if err != nil {
		return syncHookOutput{}, 0, "", err
	}
	out, parseErr := parseSyncOutput(stdout)
	return out, code, firstJSONLine(stdout), parseErr
}

func (b binding) spawn(ctx context.Context, in hookInput) ([]byte, int, error) {
	timeout := defaultTimeout
	if b.cmd.Timeout > 0 {
		timeout = min(time.Duration(b.cmd.Timeout)*time.Second, maxTimeout)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := json.Marshal(in)
	if err != nil {
		return nil, 0, err
	}
	payload = append(payload, '\n')

	shell, args := shellInvocation(b.cmd.Shell, b.cmd.Command)
	cmd := exec.CommandContext(ctx, shell, args...) //nolint:gosec // G204: user-configured hook commands
	cmd.Dir = b.dir
	cmd.Env = sanitizeEnv(os.Environ(), hookEnv{
		Event:      in.HookEventName,
		SessionID:  in.SessionID,
		Cwd:        in.Cwd,
		ProjectDir: in.Cwd,
	})
	if b.dir != "" {
		cmd.Env = append(cmd.Env, pluginRootEnvKey+"="+b.dir)
	}
	cmd.Stdin = bytes.NewReader(payload)

	var stdout, stderr limitedBuffer
	stdout.limit = maxHookOutput
	stderr.limit = maxHookOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return stdout.Bytes(), 0, fmt.Errorf("hook timed out after %s", timeout)
	}
	if err != nil {
		if ee, ok := errors.AsType[*exec.ExitError](err); ok {
			return stdout.Bytes(), ee.ExitCode(), nil
		}
		return stdout.Bytes(), 0, err
	}
	return stdout.Bytes(), 0, nil
}

func shellInvocation(shell, command string) (string, []string) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "powershell", "pwsh":
		if path, err := exec.LookPath("pwsh"); err == nil {
			return path, []string{"-NoProfile", "-Command", command}
		}
		return "powershell", []string{"-NoProfile", "-Command", command}
	default:
		sh := strings.TrimSpace(os.Getenv("SHELL"))
		if sh == "" {
			sh = "/bin/sh"
		}
		return sh, []string{"-c", command}
	}
}

func parseSyncOutput(stdout []byte) (syncHookOutput, error) {
	line := firstJSONLine(stdout)
	if line == "" {
		return syncHookOutput{}, nil
	}
	var out syncHookOutput
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		return syncHookOutput{}, fmt.Errorf("invalid json: %w", err)
	}
	return out, nil
}

func applyPreOutput(out *PreOutcome, sync syncHookOutput, code int, rawLine string) {
	if code == exitBlocking {
		out.Denied = true
		if sync.StopReason != "" {
			out.Reason = sync.StopReason
		} else if sync.Reason != "" {
			out.Reason = sync.Reason
		} else {
			out.Reason = "hook denied (exit 2)"
		}
	}
	if sync.Continue != nil && !*sync.Continue && !out.Denied {
		out.Denied = true
		if sync.StopReason != "" {
			out.Reason = sync.StopReason
		}
	}
	if strings.ToLower(strings.TrimSpace(sync.Decision)) == "block" {
		out.Denied = true
		if sync.Reason != "" {
			out.Reason = sync.Reason
		}
	}

	if len(sync.HookSpecificOutput) > 0 {
		var spec preToolSpecific
		if json.Unmarshal(sync.HookSpecificOutput, &spec) == nil {
			switch strings.ToLower(strings.TrimSpace(spec.PermissionDecision)) {
			case "deny":
				out.Denied = true
				if spec.PermissionDecisionReason != "" {
					out.Reason = spec.PermissionDecisionReason
				}
			case "allow", "ask":
				// ask falls through to Gate
			}
			if len(spec.UpdatedInput) > 0 {
				out.Input = spec.UpdatedInput
			}
			if spec.AdditionalContext != "" {
				out.Context = joinContextParts(out.Context, spec.AdditionalContext)
			}
		}
	}
	if rawLine != "" && len(sync.HookSpecificOutput) == 0 && strings.TrimSpace(sync.Decision) == "" {
		applyLegacyPre(rawLine, out)
	}
}

func applyLegacyPre(rawLine string, out *PreOutcome) {
	var leg struct {
		Action  string          `json:"action"`
		Reason  string          `json:"reason"`
		Input   json.RawMessage `json:"input"`
		Context string          `json:"context"`
	}
	if err := json.Unmarshal([]byte(rawLine), &leg); err != nil || leg.Action == "" {
		return
	}
	switch strings.ToLower(leg.Action) {
	case "deny":
		out.Denied = true
		out.Reason = leg.Reason
	case "modify":
		if len(leg.Input) > 0 {
			out.Input = leg.Input
		}
	}
	if leg.Context != "" {
		out.Context = joinContextParts(out.Context, leg.Context)
	}
}

func applyPostOutput(out *PostOutcome, sync syncHookOutput, code int, rawLine string) {
	if code == exitBlocking {
		out.Stop = true
		if sync.StopReason != "" {
			out.Reason = sync.StopReason
		} else {
			out.Reason = "hook denied (exit 2)"
		}
	}
	if sync.Continue != nil && !*sync.Continue {
		out.Stop = true
		if sync.StopReason != "" {
			out.Reason = sync.StopReason
		}
	}

	if len(sync.HookSpecificOutput) > 0 {
		var spec postToolSpecific
		if json.Unmarshal(sync.HookSpecificOutput, &spec) == nil {
			if spec.AdditionalContext != "" {
				out.Context = joinContextParts(out.Context, spec.AdditionalContext)
			}
			if spec.UpdatedMCPToolOutput != "" {
				out.Output = spec.UpdatedMCPToolOutput
			}
		}
	}
	if rawLine != "" && len(sync.HookSpecificOutput) == 0 {
		applyLegacyPost(rawLine, out)
	}
}

func applyLegacyPost(rawLine string, out *PostOutcome) {
	var leg struct {
		Context string `json:"context"`
		Output  string `json:"output"`
		Stop    bool   `json:"stop"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(rawLine), &leg); err != nil {
		return
	}
	out.Context = leg.Context
	out.Output = leg.Output
	out.Stop = leg.Stop
	out.Reason = leg.Reason
}

func applySessionStartOutput(out *SessionOutcome, sync syncHookOutput) {
	if sync.SystemMessage != "" {
		out.Toast = sync.SystemMessage
	}
	if len(sync.HookSpecificOutput) == 0 {
		return
	}
	var spec sessionStartSpecific
	if json.Unmarshal(sync.HookSpecificOutput, &spec) != nil {
		return
	}
	if spec.InitialUserMessage != "" {
		out.Toast = spec.InitialUserMessage
	}
}

func applySessionGateOutput(out *SessionOutcome, sync syncHookOutput, code int, rawLine string) {
	applySessionStartOutput(out, sync)
	if code == exitBlocking {
		out.Denied = true
		if sync.StopReason != "" {
			out.Reason = sync.StopReason
		} else if sync.Reason != "" {
			out.Reason = sync.Reason
		} else {
			out.Reason = "hook denied (exit 2)"
		}
	}
	if sync.Continue != nil && !*sync.Continue && !out.Denied {
		out.Denied = true
		if sync.StopReason != "" {
			out.Reason = sync.StopReason
		}
	}
	if rawLine != "" {
		applyLegacySessionGate(rawLine, out)
	}
}

func applyLegacySessionGate(rawLine string, out *SessionOutcome) {
	var leg struct {
		Action string  `json:"action"`
		Reason string  `json:"reason"`
		Toast  string  `json:"toast"`
		Status *string `json:"status"`
	}
	if err := json.Unmarshal([]byte(rawLine), &leg); err != nil {
		return
	}
	if strings.EqualFold(strings.TrimSpace(leg.Action), "deny") {
		out.Denied = true
		if leg.Reason != "" {
			out.Reason = leg.Reason
		}
	}
	if leg.Toast != "" {
		out.Toast = leg.Toast
	}
	if leg.Status != nil {
		out.Status = *leg.Status
		out.StatusSet = true
	}
}

func firstJSONLine(b []byte) string {
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

type limitedBuffer struct {
	buf   bytes.Buffer
	limit int
	n     int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.limit <= 0 {
		return l.buf.Write(p)
	}
	remain := l.limit - l.n
	if remain <= 0 {
		return len(p), nil
	}
	if len(p) > remain {
		l.buf.Write(p[:remain])
		l.n = l.limit
		return len(p), nil
	}
	n, err := l.buf.Write(p)
	l.n += n
	return n, err
}

func (l *limitedBuffer) Bytes() []byte { return l.buf.Bytes() }

var _ io.Writer = (*limitedBuffer)(nil)
