package sdk

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/pulseaiclub/phi/ext"
	"github.com/pulseaiclub/phi/ext/pxb"
)

// Module is the author-facing registration surface for a PXB extension binary.
type Module struct {
	Name    string
	Version string

	mu        sync.Mutex
	tools     []toolReg
	commands  []cmdReg
	events    []uint16
	intercept []uint16

	onToolCall            func(ext.ToolCallEvent) *ext.ToolCallResult
	onToolResult          func(ext.ToolResultEvent) *ext.ToolResultResult
	onBeforeAgentStart    func(ext.BeforeAgentStartEvent) *ext.BeforeAgentStartResult
	onSessionBeforeSwitch func(ext.SessionBeforeSwitchEvent) *ext.SessionBeforeSwitchResult
	onEvent               map[uint16]func(pxb.EventNotify)

	wr *pxb.Writer
	rd *pxb.Reader

	host          HelloInfo
	pendingSubmit string
}

type toolReg struct {
	def ext.Tool
}

type cmdReg struct {
	name string
	def  ext.Command
}

// HelloInfo is filled after hello_ack.
type HelloInfo struct {
	Cwd          string
	SessionID    string
	ExtensionDir string
	PhiVersion   string
}

// New constructs a module.
func New(name, version string) *Module {
	return &Module{
		Name:    name,
		Version: version,
		onEvent: make(map[uint16]func(pxb.EventNotify)),
	}
}

// Host returns metadata from the host handshake.
func (m *Module) Host() HelloInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.host
}

// RegisterTool adds an LLM-callable tool.
func (m *Module) RegisterTool(t ext.Tool) {
	if t.Name == "" || t.Execute == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = append(m.tools, toolReg{def: t})
}

// RegisterCommand adds a slash command.
func (m *Module) RegisterCommand(name string, cmd ext.Command) {
	if name == "" || cmd.Handler == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commands = append(m.commands, cmdReg{name: name, def: cmd})
}

// OnToolCall registers a pre-gate intercept.
func (m *Module) OnToolCall(fn func(ext.ToolCallEvent) *ext.ToolCallResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onToolCall = fn
	m.intercept = appendUnique(m.intercept, pxb.EvToolCall)
}

// OnToolResult registers a post-tool intercept.
func (m *Module) OnToolResult(fn func(ext.ToolResultEvent) *ext.ToolResultResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onToolResult = fn
	m.intercept = appendUnique(m.intercept, pxb.EvToolResult)
}

// OnBeforeAgentStart may append system prompt text.
func (m *Module) OnBeforeAgentStart(fn func(ext.BeforeAgentStartEvent) *ext.BeforeAgentStartResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onBeforeAgentStart = fn
	m.intercept = appendUnique(m.intercept, pxb.EvBeforeAgentStart)
}

// OnSessionBeforeSwitch may cancel a session switch.
func (m *Module) OnSessionBeforeSwitch(fn func(ext.SessionBeforeSwitchEvent) *ext.SessionBeforeSwitchResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSessionBeforeSwitch = fn
	m.intercept = appendUnique(m.intercept, pxb.EvSessionBeforeSwitch)
}

// Subscribe adds a fire-and-forget lifecycle listener.
func (m *Module) Subscribe(event string, fn func()) {
	code := pxb.EventCode(event)
	if code == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = appendUnique(m.events, code)
	if fn != nil {
		m.onEvent[code] = func(pxb.EventNotify) { fn() }
	}
}

// Notify pushes a toast to the host (after Run has started).
func (m *Module) Notify(level, message string) {
	if m.wr == nil {
		return
	}
	_ = m.wr.Write(pxb.TypeNotify, 0, 0, pxb.EncodeNotify(pxb.NotifyMsg{Level: level, Message: message}))
}

// SetStatus updates the host footer extension status (empty clears).
func (m *Module) SetStatus(text string) {
	if m.wr == nil {
		return
	}
	_ = m.wr.Write(pxb.TypeNotify, 0, 0, pxb.EncodeNotify(pxb.NotifyMsg{Status: text, StatusSet: true}))
}

// Submit queues a prompt for the host to send after the current slash command returns.
func (m *Module) Submit(text string) {
	m.mu.Lock()
	m.pendingSubmit = text
	m.mu.Unlock()
}

// moduleUI implements ext.UI over PXB Notify frames.
type moduleUI struct{ m *Module }

func (u moduleUI) Notify(message, kind string) { u.m.Notify(kind, message) }
func (u moduleUI) SetStatus(_, text string)    { u.m.SetStatus(text) }
func (moduleUI) Confirm(_, _ string) bool      { return false }

// Run speaks PXB on stdin/stdout until shutdown.
func (m *Module) Run() error {
	m.wr = pxb.NewWriter(os.Stdout)
	m.rd = pxb.NewReader(os.Stdin)

	caps := uint32(0)
	m.mu.Lock()
	if len(m.commands) > 0 {
		caps |= pxb.CapCommands
	}
	if len(m.tools) > 0 {
		caps |= pxb.CapTools
	}
	if len(m.events) > 0 {
		caps |= pxb.CapEvents
	}
	if len(m.intercept) > 0 {
		caps |= pxb.CapIntercept
	}
	hello := pxb.EncodeHello(pxb.Hello{
		Name: m.Name, Version: m.Version, Caps: caps, Protocol: pxb.ProtocolVersion,
	})
	m.mu.Unlock()

	if err := m.wr.Write(pxb.TypeHello, 0, 0, hello); err != nil {
		return err
	}

	f, err := m.rd.Read()
	if err != nil {
		return err
	}
	if f.Type != pxb.TypeHelloAck {
		return errUnexpected("hello_ack", f.Type)
	}
	ack, err := pxb.DecodeHelloAck(pxb.CloneBody(f))
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.host = HelloInfo{
		Cwd: ack.Cwd, SessionID: ack.SessionID,
		ExtensionDir: ack.ExtensionDir, PhiVersion: ack.PhiVersion,
	}
	tools := append([]toolReg(nil), m.tools...)
	cmds := append([]cmdReg(nil), m.commands...)
	events := append([]uint16(nil), m.events...)
	intercept := append([]uint16(nil), m.intercept...)
	m.mu.Unlock()

	for _, t := range tools {
		schema, _ := json.Marshal(t.def.Parameters)
		body := pxb.EncodeRegisterTool(pxb.RegisterTool{
			Name: t.def.Name, Description: t.def.Description, SchemaJSON: schema,
		})
		if err := m.wr.Write(pxb.TypeRegisterTool, 0, 0, body); err != nil {
			return err
		}
	}
	for _, c := range cmds {
		body := pxb.EncodeRegisterCommand(pxb.RegisterCommand{
			Name: c.name, Description: c.def.Description,
		})
		if err := m.wr.Write(pxb.TypeRegisterCommand, 0, 0, body); err != nil {
			return err
		}
	}
	if len(events) > 0 || len(intercept) > 0 {
		body := pxb.EncodeSubscribe(pxb.Subscribe{Events: events, Intercept: intercept})
		if err := m.wr.Write(pxb.TypeSubscribe, 0, 0, body); err != nil {
			return err
		}
	}
	if err := m.wr.Write(pxb.TypeReady, 0, 0, nil); err != nil {
		return err
	}

	toolByName := make(map[string]ext.Tool, len(tools))
	for _, t := range tools {
		toolByName[t.def.Name] = t.def
	}
	cmdByName := make(map[string]ext.Command, len(cmds))
	for _, c := range cmds {
		cmdByName[c.name] = c.def
	}

	var running atomic.Bool
	running.Store(true)
	for running.Load() {
		fr, err := m.rd.Read()
		if err != nil {
			return err
		}
		body := pxb.CloneBody(fr)
		switch fr.Type {
		case pxb.TypeShutdown:
			_ = m.wr.Write(pxb.TypeShutdownAck, 0, 0, nil)
			running.Store(false)
		case pxb.TypeCommandInvoked:
			inv, err := pxb.DecodeCommandInvoked(body)
			if err != nil {
				return err
			}
			resp := pxb.CommandResponse{OK: true}
			if cmd, ok := cmdByName[inv.Name]; ok {
				if err := cmd.Handler(inv.Args, &ext.Context{
					Cwd: m.host.Cwd, SessionID: m.host.SessionID, HasUI: true, UI: moduleUI{m: m},
				}); err != nil {
					resp.OK = false
					resp.Error = err.Error()
				}
			} else {
				resp.OK = false
				resp.Error = "unknown command"
			}
			m.mu.Lock()
			resp.Submit = m.pendingSubmit
			m.pendingSubmit = ""
			m.mu.Unlock()
			_ = m.wr.Write(pxb.TypeCommandResponse, fr.Flags, fr.ID, pxb.EncodeCommandResponse(resp))
		case pxb.TypeToolInvoke:
			inv, err := pxb.DecodeToolInvoke(body)
			if err != nil {
				return err
			}
			tr := pxb.ToolResultMsg{}
			if tool, ok := toolByName[inv.Name]; ok {
				res, err := tool.Execute(context.Background(), inv.Args)
				if err != nil {
					tr.IsError = true
					tr.Error = err.Error()
					tr.Content = err.Error()
				} else {
					tr.Content, tr.Detail, tr.Output = res.Content, res.Detail, res.Output
				}
			} else {
				tr.IsError = true
				tr.Error = "unknown tool"
			}
			_ = m.wr.Write(pxb.TypeToolResult, fr.Flags, fr.ID, pxb.EncodeToolResult(tr))
		case pxb.TypeIntercept:
			req, err := pxb.DecodeInterceptReq(body)
			if err != nil {
				return err
			}
			resp := m.handleIntercept(req)
			_ = m.wr.Write(pxb.TypeInterceptResponse, fr.Flags, fr.ID, pxb.EncodeInterceptResp(resp))
		case pxb.TypeEvent:
			ev, err := pxb.DecodeEventNotify(body)
			if err != nil {
				return err
			}
			m.mu.Lock()
			fn := m.onEvent[ev.Event]
			m.mu.Unlock()
			if fn != nil {
				fn(ev)
			}
		}
	}
	return nil
}

func (m *Module) handleIntercept(req pxb.InterceptReq) pxb.InterceptResp {
	switch req.Event {
	case pxb.EvToolCall:
		if m.onToolCall == nil {
			return pxb.InterceptResp{}
		}
		r := m.onToolCall(ext.ToolCallEvent{
			ToolName: req.ToolName, ToolCallID: req.ToolCallID, Input: req.Input,
		})
		if r == nil {
			return pxb.InterceptResp{}
		}
		return pxb.InterceptResp{Block: r.Block, Reason: r.Reason, Input: r.Input, Context: r.Context}
	case pxb.EvToolResult:
		if m.onToolResult == nil {
			return pxb.InterceptResp{}
		}
		r := m.onToolResult(ext.ToolResultEvent{
			ToolName: req.ToolName, ToolCallID: req.ToolCallID, Input: req.Input,
			Content: req.Content, IsError: req.IsError, Err: req.ErrText,
		})
		if r == nil {
			return pxb.InterceptResp{}
		}
		return pxb.InterceptResp{Content: r.Content, Context: r.Context, Stop: r.Stop, Reason: r.Reason}
	case pxb.EvBeforeAgentStart:
		if m.onBeforeAgentStart == nil {
			return pxb.InterceptResp{}
		}
		r := m.onBeforeAgentStart(ext.BeforeAgentStartEvent{Prompt: req.Prompt})
		if r == nil {
			return pxb.InterceptResp{}
		}
		return pxb.InterceptResp{SystemPromptAppend: r.SystemPromptAppend}
	case pxb.EvSessionBeforeSwitch:
		if m.onSessionBeforeSwitch == nil {
			return pxb.InterceptResp{}
		}
		r := m.onSessionBeforeSwitch(ext.SessionBeforeSwitchEvent{
			Reason: req.Reason, TargetSessionID: req.TargetID,
		})
		if r == nil {
			return pxb.InterceptResp{}
		}
		return pxb.InterceptResp{Cancel: r.Cancel, Reason: r.Reason, Toast: r.Toast}
	default:
		return pxb.InterceptResp{}
	}
}

func appendUnique(xs []uint16, v uint16) []uint16 {
	if slices.Contains(xs, v) {
		return xs
	}
	return append(xs, v)
}

type unexpectedTypeError struct {
	want string
	got  uint16
}

func (e unexpectedTypeError) Error() string {
	return "sdk: expected " + e.want + " frame"
}

func errUnexpected(want string, got uint16) error {
	return unexpectedTypeError{want: want, got: got}
}
