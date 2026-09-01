package sdk

import (
	"encoding/json"
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
	onUserInput           func(ext.UserInputEvent) *ext.UserInputResult
	onTurnStopping        func(ext.TurnStoppingEvent) *ext.TurnStoppingResult
	onEvent               map[uint16]func(pxb.EventNotify)

	wr *pxb.Writer
	rd *pxb.Reader

	host          HelloInfo
	pendingSubmit string
	nextHostID    atomic.Uint32
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

// OnUserInput may transform or swallow the user prompt before the agent loop.
func (m *Module) OnUserInput(fn func(ext.UserInputEvent) *ext.UserInputResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onUserInput = fn
	m.intercept = appendUnique(m.intercept, pxb.EvUserInput)
}

// OnTurnStopping may steer another step when the model stops with no tools.
func (m *Module) OnTurnStopping(fn func(ext.TurnStoppingEvent) *ext.TurnStoppingResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTurnStopping = fn
	m.intercept = appendUnique(m.intercept, pxb.EvTurnStopping)
}

// Subscribe adds a fire-and-forget lifecycle listener (no payload).
func (m *Module) Subscribe(event string, fn func()) {
	m.SubscribeEvent(event, func(pxb.EventNotify) {
		if fn != nil {
			fn()
		}
	})
}

// SubscribeEvent adds a fire-and-forget listener with the wire payload.
func (m *Module) SubscribeEvent(event string, fn func(pxb.EventNotify)) {
	code := pxb.EventCode(event)
	if code == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = appendUnique(m.events, code)
	if fn != nil {
		m.onEvent[code] = fn
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

// SendUserMessage asks the host to enqueue a user turn (fire-and-forget).
// Safe to call from command/tool handlers on the PXB read loop.
func (m *Module) SendUserMessage(text string) {
	if m.wr == nil || text == "" {
		return
	}
	_ = m.wr.Write(pxb.TypeHostRequest, 0, 0, pxb.EncodeHostRequest(pxb.HostRequest{
		Method: "send_user_message", Arg: text,
	}))
}

// Confirm shows a yes/no dialog on the host and waits for the answer.
// Must be called from a command/tool/intercept handler (nested read on the PXB loop).
func (m *Module) Confirm(title, message string) bool {
	return m.ConfirmOpts(ext.ConfirmRequest{Title: title, Message: message}).OK
}

// ConfirmOpts is Confirm with labels / danger styling.
func (m *Module) ConfirmOpts(req ext.ConfirmRequest) ext.ConfirmReply {
	if m.wr == nil || m.rd == nil {
		return ext.ConfirmReply{}
	}
	payload, _ := json.Marshal(req)
	id := m.nextHostID.Add(1)
	if err := m.wr.Write(pxb.TypeHostRequest, pxb.FlagHasID, id, pxb.EncodeHostRequest(pxb.HostRequest{
		Method: "confirm", Arg: string(payload),
	})); err != nil {
		return ext.ConfirmReply{}
	}
	for {
		fr, err := m.rd.Read()
		if err != nil {
			return ext.ConfirmReply{}
		}
		body := pxb.CloneBody(fr)
		switch fr.Type {
		case pxb.TypeHostResult:
			if fr.Flags&pxb.FlagHasID == 0 || fr.ID != id {
				continue
			}
			res, err := pxb.DecodeHostResult(body)
			if err != nil {
				return ext.ConfirmReply{}
			}
			return ext.ConfirmReply{OK: res.OK}
		case pxb.TypeSessionMeta:
			meta, err := pxb.DecodeSessionMeta(body)
			if err != nil {
				continue
			}
			m.mu.Lock()
			if meta.SessionID != "" {
				m.host.SessionID = meta.SessionID
			}
			if meta.Cwd != "" {
				m.host.Cwd = meta.Cwd
			}
			m.mu.Unlock()
		case pxb.TypeEvent:
			ev, err := pxb.DecodeEventNotify(body)
			if err != nil {
				continue
			}
			m.mu.Lock()
			fn := m.onEvent[ev.Event]
			m.mu.Unlock()
			if fn != nil {
				fn(ev)
			}
		case pxb.TypeShutdown:
			_ = m.wr.Write(pxb.TypeShutdownAck, 0, 0, nil)
			return ext.ConfirmReply{}
		default:
			// Ignore unrelated frames while blocked on confirm.
		}
	}
}

// ShowPane opens or replaces a non-blocking extension pane on the host.
func (m *Module) ShowPane(p ext.Pane) {
	if m.wr == nil {
		return
	}
	if p.ID == "" {
		p.ID = "default"
	}
	payload, _ := json.Marshal(p)
	_ = m.wr.Write(pxb.TypeHostRequest, 0, 0, pxb.EncodeHostRequest(pxb.HostRequest{
		Method: "pane_show", Arg: string(payload),
	}))
}

// UpdatePane replaces the body of an existing pane.
func (m *Module) UpdatePane(id, body string) {
	if m.wr == nil {
		return
	}
	if id == "" {
		id = "default"
	}
	payload, _ := json.Marshal(map[string]string{"id": id, "body": body})
	_ = m.wr.Write(pxb.TypeHostRequest, 0, 0, pxb.EncodeHostRequest(pxb.HostRequest{
		Method: "pane_update", Arg: string(payload),
	}))
}

// ClosePane dismisses a pane.
func (m *Module) ClosePane(id string) {
	if m.wr == nil {
		return
	}
	if id == "" {
		id = "default"
	}
	payload, _ := json.Marshal(map[string]string{"id": id})
	_ = m.wr.Write(pxb.TypeHostRequest, 0, 0, pxb.EncodeHostRequest(pxb.HostRequest{
		Method: "pane_close", Arg: string(payload),
	}))
}

// OnPaneAction registers a listener for pane button clicks.
func (m *Module) OnPaneAction(fn func(paneID, actionID string)) {
	if fn == nil {
		return
	}
	m.SubscribeEvent(ext.EventPaneAction, func(ev pxb.EventNotify) {
		fn(ev.Prompt, ev.Reason)
	})
}
