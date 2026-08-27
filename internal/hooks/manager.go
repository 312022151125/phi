package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/pulseaiclub/phi/internal/debuglog"
)

// MaxContextBytes caps aggregated hook context injected back to the model.
const MaxContextBytes = 4 * 1024

// Manager fans hook bindings across tool and session events.
// A nil *Manager is safe and is a no-op.
type Manager struct {
	bindings []binding
}

// NewManager builds a manager from discovered plugins.
func NewManager(ds ...Discovered) *Manager {
	var bindings []binding
	for _, d := range ds {
		for _, h := range d.Hooks {
			for _, m := range h.Matchers {
				for _, cmd := range m.Hooks {
					bindings = append(bindings, binding{
						pluginID: d.Plugin,
						dir:      h.Dir,
						event:    h.Event,
						matcher:  m.Matcher,
						cmd:      cmd,
					})
				}
			}
		}
	}
	return &Manager{bindings: bindings}
}

// PreTool runs PreToolUse bindings serially. First deny wins; updatedInput chains.
func (m *Manager) PreTool(ctx context.Context, ev ToolEvent) PreOutcome {
	if m == nil {
		return PreOutcome{Input: ev.Input}
	}
	out := PreOutcome{Input: ev.Input}
	for _, b := range m.bindings {
		if b.event != EventPreToolUse {
			continue
		}
		if !matchesPattern(b.matcher, ev.Tool) {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		if b.cmd.Async {
			go m.runPreAsync(b, ev) //nolint:gosec // G118: async hooks detach from tool ctx
			continue
		}

		callEv := ev
		callEv.Input = out.Input
		sync, code, rawLine, err := b.run(ctx, preToolInput(callEv))
		if err != nil {
			debuglog.Logf("hooks: %s PreToolUse: %v", b.pluginID, err)
			continue
		}
		if code != 0 && code != exitBlocking {
			debuglog.Logf("hooks: %s PreToolUse exited %d", b.pluginID, code)
			continue
		}
		applyPreOutput(&out, sync, code, rawLine)
		if out.Denied {
			out.Context = capContext(out.Context)
			return out
		}
	}
	out.Context = capContext(out.Context)
	return out
}

// PostTool runs PostToolUse (success) or PostToolUseFailure (tool error) bindings.
func (m *Manager) PostTool(ctx context.Context, ev ToolEvent) PostOutcome {
	if m == nil {
		return PostOutcome{}
	}

	event := EventPostToolUse
	if ev.Err != "" {
		event = EventPostToolUseFailure
	}

	type result struct {
		out  PostOutcome
		code int
		err  error
		b    binding
	}

	var syncBindings []binding
	for _, b := range m.bindings {
		if b.event != event {
			continue
		}
		if !matchesPattern(b.matcher, ev.Tool) {
			continue
		}
		if b.cmd.Async {
			go m.runPostAsync(b, ev, event) //nolint:gosec // G118: async hooks detach from tool ctx
			continue
		}
		syncBindings = append(syncBindings, b)
	}
	if len(syncBindings) == 0 {
		return PostOutcome{}
	}

	var wg sync.WaitGroup
	results := make([]result, len(syncBindings))
	for i, b := range syncBindings {
		wg.Add(1)
		go func(i int, b binding) {
			defer wg.Done()
			in := postToolInput(ev, event)
			sync, code, rawLine, err := b.run(ctx, in)
			r := result{b: b, code: code, err: err}
			if err == nil {
				applyPostOutput(&r.out, sync, code, rawLine)
			}
			results[i] = r
		}(i, b)
	}
	wg.Wait()

	var (
		contexts []string
		reasons  []string
		output   string
		stop     bool
	)
	for _, r := range results {
		if r.err != nil {
			debuglog.Logf("hooks: %s %s: %v", r.b.pluginID, event, r.err)
			continue
		}
		if r.code != 0 && r.code != exitBlocking {
			debuglog.Logf("hooks: %s %s exited %d", r.b.pluginID, event, r.code)
			continue
		}
		if r.out.Context != "" {
			contexts = append(contexts, r.out.Context)
		}
		if r.out.Output != "" {
			output = r.out.Output
		}
		if r.out.Stop {
			stop = true
			if r.out.Reason != "" {
				reasons = append(reasons, r.out.Reason)
			}
		}
	}

	return PostOutcome{
		Context: capContext(joinContextParts(contexts...)),
		Output:  output,
		Stop:    stop,
		Reason:  strings.Join(reasons, "; "),
	}
}

// SessionStart runs SessionStart bindings (matcher = source).
func (m *Manager) SessionStart(ctx context.Context, ev SessionEvent) SessionOutcome {
	return m.runSession(ctx, EventSessionStart, ev.Reason, ev)
}

// SessionShutdown runs when the active session is left (new / resume / quit).
// Matcher filters on reason. Not a CC SessionEnd — the session may still exist on disk.
func (m *Manager) SessionShutdown(ctx context.Context, ev SessionEvent) SessionOutcome {
	return m.runSession(ctx, EventSessionShutdown, ev.Reason, ev)
}

// SessionBeforeSwitch runs SessionBeforeSwitch bindings serially. First deny wins.
func (m *Manager) SessionBeforeSwitch(ctx context.Context, ev SessionEvent) SessionOutcome {
	return m.runSessionGate(ctx, EventSessionBeforeSwitch, ev.Reason, ev)
}

// PostTurn runs PostTurn bindings (parallel; async detached). Audit-only.
func (m *Manager) PostTurn(ctx context.Context, ev SessionEvent) {
	m.runSessionAudit(ctx, EventPostTurn, ev)
}

func (m *Manager) runSession(ctx context.Context, event HookEvent, matchQuery string, ev SessionEvent) SessionOutcome {
	if m == nil {
		return SessionOutcome{}
	}

	type result struct {
		out SessionOutcome
		err error
		b   binding
	}

	var syncBindings []binding
	for _, b := range m.bindings {
		if b.event != event {
			continue
		}
		if !matchesPattern(b.matcher, matchQuery) {
			continue
		}
		if b.cmd.Async {
			go m.runSessionAsync(b, ev, event, matchQuery) //nolint:gosec // G118: async session hooks detach
			continue
		}
		syncBindings = append(syncBindings, b)
	}
	if len(syncBindings) == 0 {
		return SessionOutcome{}
	}

	var wg sync.WaitGroup
	results := make([]result, len(syncBindings))
	for i, b := range syncBindings {
		wg.Add(1)
		go func(i int, b binding) {
			defer wg.Done()
			in := sessionInput(ev, event, matchQuery)
			sync, code, _, err := b.run(ctx, in)
			r := result{b: b, err: err}
			if err == nil && (code == 0 || code == exitBlocking) {
				applySessionStartOutput(&r.out, sync)
			}
			results[i] = r
		}(i, b)
	}
	wg.Wait()

	var out SessionOutcome
	for _, r := range results {
		if r.err != nil {
			debuglog.Logf("hooks: %s %s: %v", r.b.pluginID, event, r.err)
			continue
		}
		mergeSessionUI(&out, r.out)
	}
	return out
}

func (m *Manager) runSessionGate(
	ctx context.Context,
	event HookEvent,
	matchQuery string,
	ev SessionEvent,
) SessionOutcome {
	if m == nil {
		return SessionOutcome{}
	}
	var out SessionOutcome
	for _, b := range m.bindings {
		if b.event != event {
			continue
		}
		if !matchesPattern(b.matcher, matchQuery) {
			continue
		}
		if ctx.Err() != nil {
			break
		}
		if b.cmd.Async {
			go m.runSessionAsync(b, ev, event, matchQuery) //nolint:gosec // G118: async session hooks detach
			continue
		}
		sync, code, rawLine, err := b.run(ctx, sessionInput(ev, event, matchQuery))
		if err != nil {
			debuglog.Logf("hooks: %s %s: %v", b.pluginID, event, err)
			continue
		}
		if code != 0 && code != exitBlocking {
			debuglog.Logf("hooks: %s %s exited %d", b.pluginID, event, code)
			continue
		}
		applySessionGateOutput(&out, sync, code, rawLine)
		if out.Denied {
			if out.Reason == "" {
				out.Reason = "session switch denied by hook " + b.pluginID
			}
			return out
		}
	}
	return out
}

func (m *Manager) runSessionAudit(ctx context.Context, event HookEvent, ev SessionEvent) {
	if m == nil {
		return
	}

	type result struct {
		err error
		b   binding
	}

	var syncBindings []binding
	for _, b := range m.bindings {
		if b.event != event {
			continue
		}
		if b.cmd.Async {
			go m.runSessionAsync(b, ev, event, ev.Reason) //nolint:gosec // G118: async audit hooks detach
			continue
		}
		syncBindings = append(syncBindings, b)
	}
	if len(syncBindings) == 0 {
		return
	}

	var wg sync.WaitGroup
	results := make([]result, len(syncBindings))
	for i, b := range syncBindings {
		wg.Add(1)
		go func(i int, b binding) {
			defer wg.Done()
			_, code, _, err := b.run(ctx, sessionInput(ev, event, ev.Reason))
			if err != nil {
				results[i] = result{b: b, err: err}
				return
			}
			if code != 0 && code != exitBlocking {
				results[i] = result{b: b, err: fmt.Errorf("exited %d", code)}
			}
		}(i, b)
	}
	wg.Wait()

	for _, r := range results {
		if r.err != nil {
			debuglog.Logf("hooks: %s %s: %v", r.b.pluginID, event, r.err)
		}
	}
}

func (*Manager) runPreAsync(b binding, ev ToolEvent) {
	callEv := ev
	if _, _, _, err := b.run(context.Background(), preToolInput(callEv)); err != nil {
		debuglog.Logf("hooks: %s PreToolUse async: %v", b.pluginID, err)
	}
}

func (*Manager) runPostAsync(b binding, ev ToolEvent, event HookEvent) {
	if _, _, _, err := b.run(context.Background(), postToolInput(ev, event)); err != nil {
		debuglog.Logf("hooks: %s %s async: %v", b.pluginID, event, err)
	}
}

func (*Manager) runSessionAsync(b binding, ev SessionEvent, event HookEvent, matchQuery string) {
	if _, _, _, err := b.run(context.Background(), sessionInput(ev, event, matchQuery)); err != nil {
		debuglog.Logf("hooks: %s %s async: %v", b.pluginID, event, err)
	}
}

func preToolInput(ev ToolEvent) hookInput {
	return hookInput{
		SessionID:     ev.SessionID,
		Cwd:           ev.Cwd,
		HookEventName: string(EventPreToolUse),
		ToolName:      ev.Tool,
		ToolInput:     ev.Input,
		ToolUseID:     ev.ToolUseID,
	}
}

func postToolInput(ev ToolEvent, event HookEvent) hookInput {
	in := hookInput{
		SessionID:     ev.SessionID,
		Cwd:           ev.Cwd,
		HookEventName: string(event),
		ToolName:      ev.Tool,
		ToolInput:     ev.Input,
		ToolUseID:     ev.ToolUseID,
		Error:         ev.Err,
	}
	if event == EventPostToolUse {
		in.ToolResponse = ev.Output
	}
	return in
}

func sessionInput(ev SessionEvent, event HookEvent, matchQuery string) hookInput {
	in := hookInput{
		SessionID:         ev.SessionID,
		Cwd:               ev.Cwd,
		HookEventName:     string(event),
		PreviousSessionID: ev.PreviousSessionID,
		TargetSessionID:   ev.TargetSessionID,
	}
	switch event {
	case EventSessionStart:
		in.Source = matchQuery
	case EventSessionShutdown, EventSessionBeforeSwitch:
		in.Reason = matchQuery
	case EventPostTurn:
		in.MessageID = ev.MessageID
		in.Usage = usageInput(ev.Usage)
	}
	return in
}

func usageInput(u SessionUsage) *hookUsage {
	if u == (SessionUsage{}) {
		return nil
	}
	return &hookUsage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		CachedTokens:     u.CachedTokens,
		TotalTokens:      u.TotalTokens,
	}
}

func mergeSessionUI(out *SessionOutcome, res SessionOutcome) {
	if res.Toast != "" {
		out.Toast = res.Toast
	}
	if res.StatusSet {
		out.Status = res.Status
		out.StatusSet = true
	}
}

func joinContextParts(parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

func capContext(s string) string {
	if len(s) <= MaxContextBytes {
		return s
	}
	s = s[:MaxContextBytes]
	for s != "" && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// CommandEntries returns slash commands registered from Command hooks. Nil-safe.
func (m *Manager) CommandEntries() []CommandEntry {
	if m == nil {
		return nil
	}
	seen := make(map[string]string) // lower → display name
	for _, b := range m.bindings {
		if b.event != EventCommand {
			continue
		}
		name := strings.TrimSpace(b.matcher)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; !ok {
			seen[key] = name
		}
	}
	out := make([]CommandEntry, 0, len(seen))
	for _, name := range seen {
		out = append(out, CommandEntry{Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RunCommand invokes the Command hook named name (case-insensitive).
func (m *Manager) RunCommand(ctx context.Context, name string, ev CommandEvent) (CommandResult, error) {
	if m == nil {
		return CommandResult{}, errors.New("hooks: no manager")
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return CommandResult{}, errors.New("hooks: empty command name")
	}
	for _, b := range m.bindings {
		if b.event != EventCommand {
			continue
		}
		if !commandMatches(b.matcher, name) {
			continue
		}
		_, code, rawLine, err := b.run(ctx, commandInput(name, ev))
		if err != nil {
			return CommandResult{}, fmt.Errorf("hooks: %s: %w", b.matcher, err)
		}
		res, err := parseCommandResult(code, rawLine)
		if err != nil {
			return CommandResult{}, fmt.Errorf("hooks: %s: %w", b.matcher, err)
		}
		return res, nil
	}
	return CommandResult{}, fmt.Errorf("hooks: command %q is not registered", name)
}

func commandMatches(matcher, name string) bool {
	return strings.EqualFold(strings.TrimSpace(matcher), strings.TrimSpace(name))
}

func commandInput(name string, ev CommandEvent) hookInput {
	return hookInput{
		SessionID:     ev.SessionID,
		Cwd:           ev.Cwd,
		HookEventName: string(EventCommand),
		Command:       strings.TrimSpace(name),
		Args:          ev.Args,
	}
}

func parseCommandResult(code int, rawLine string) (CommandResult, error) {
	if rawLine == "" {
		if code != 0 {
			return CommandResult{}, fmt.Errorf("exited %d", code)
		}
		return CommandResult{}, nil
	}
	var out wireCommandOut
	if err := json.Unmarshal([]byte(rawLine), &out); err != nil {
		return CommandResult{}, fmt.Errorf("invalid json: %w", err)
	}
	if code != 0 {
		reason := out.Reason
		if reason == "" {
			return CommandResult{}, fmt.Errorf("exited %d", code)
		}
		return CommandResult{}, fmt.Errorf("exited %d: %s", code, reason)
	}
	res := CommandResult{Submit: out.Submit, Toast: out.Toast, List: out.List}
	if out.Status != nil {
		res.Status = *out.Status
		res.StatusSet = true
	}
	if res.List != nil && len(res.List.Items) == 0 {
		res.List = nil
	}
	return res, nil
}

type wireCommandOut struct {
	Submit string       `json:"submit"`
	Toast  string       `json:"toast"`
	Reason string       `json:"reason"`
	Status *string      `json:"status"`
	List   *CommandList `json:"list"`
}
