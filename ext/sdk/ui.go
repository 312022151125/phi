package sdk

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"sync/atomic"

	"github.com/pulseaiclub/phi/ext"
	"github.com/pulseaiclub/phi/ext/pxb"
)

// moduleUI implements ext.UI over PXB host requests / notify frames.
type moduleUI struct{ m *Module }

func (u moduleUI) Notify(message, kind string) { u.m.Notify(kind, message) }
func (u moduleUI) SetStatus(_, text string)    { u.m.SetStatus(text) }
func (u moduleUI) Confirm(title, message string) bool {
	return u.m.Confirm(title, message)
}

func (u moduleUI) ConfirmOpts(req ext.ConfirmRequest) ext.ConfirmReply {
	return u.m.ConfirmOpts(req)
}
func (u moduleUI) ShowPane(p ext.Pane)        { u.m.ShowPane(p) }
func (u moduleUI) UpdatePane(id, body string) { u.m.UpdatePane(id, body) }
func (u moduleUI) ClosePane(id string)        { u.m.ClosePane(id) }

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
		case pxb.TypeSessionMeta:
			meta, err := pxb.DecodeSessionMeta(body)
			if err != nil {
				return err
			}
			m.mu.Lock()
			if meta.SessionID != "" {
				m.host.SessionID = meta.SessionID
			}
			if meta.Cwd != "" {
				m.host.Cwd = meta.Cwd
			}
			m.mu.Unlock()
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
		return pxb.InterceptResp{SystemPromptAppend: r.SystemPromptAppend, Prompt: r.Prompt}
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
	case pxb.EvUserInput:
		if m.onUserInput == nil {
			return pxb.InterceptResp{}
		}
		r := m.onUserInput(ext.UserInputEvent{Text: req.Prompt})
		if r == nil {
			return pxb.InterceptResp{}
		}
		return pxb.InterceptResp{Handled: r.Handled, Prompt: r.Text, Reason: r.Reason}
	case pxb.EvTurnStopping:
		if m.onTurnStopping == nil {
			return pxb.InterceptResp{}
		}
		r := m.onTurnStopping(ext.TurnStoppingEvent{TurnIndex: int(req.TurnIndex)})
		if r == nil {
			return pxb.InterceptResp{}
		}
		return pxb.InterceptResp{Continue: r.Continue, Prompt: r.Message, Reason: r.Reason}
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
