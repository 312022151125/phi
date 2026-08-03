package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pulseaiclub/phi/internal/agent"
	"github.com/pulseaiclub/phi/internal/config"
	"github.com/pulseaiclub/phi/internal/session"
)

// Controller owns agent.Engine lifecycle and stream cancellation.
// It talks to the UI only by publishing Msg values onto the Bus.
type Controller struct {
	engine    *agent.Engine
	engineErr error

	streamMu     sync.Mutex
	streamCancel context.CancelFunc
	streamGen    int

	bus *Bus
}

func NewController(bus *Bus) *Controller {
	c := &Controller{bus: bus}
	if cfg, err := config.Load(); err != nil {
		c.engineErr = err
	} else {
		c.engine = agent.NewEngine(cfg)
	}
	return c
}

// SetModel rebuilds the agent engine with the given model name, keeping the
// rest of the loaded connection config (api key / base URL). Cancels any
// in-flight stream first.
func (c *Controller) SetModel(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("empty model name")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Name = name
	c.Cancel()
	c.engine = agent.NewEngine(cfg)
	c.engineErr = nil
	return nil
}

// StartPrompt cancels any in-flight stream and starts a new agent loop.
func (c *Controller) StartPrompt(text string, pendingSkills []string) {
	ctx, cancel := context.WithCancel(context.Background())
	c.streamMu.Lock()
	if c.streamCancel != nil {
		c.streamCancel()
	}
	c.streamCancel = cancel
	c.streamGen++
	gen := c.streamGen
	c.streamMu.Unlock()

	go c.runLoop(ctx, gen, text, pendingSkills)
}

// Cancel aborts the current stream context (if any).
func (c *Controller) Cancel() {
	c.streamMu.Lock()
	cancel := c.streamCancel
	c.streamMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (c *Controller) Alive(gen int) bool {
	c.streamMu.Lock()
	ok := c.streamGen == gen
	c.streamMu.Unlock()
	return ok
}

func (c *Controller) waitOrDone(ctx context.Context, gen int, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
	}
	return c.Alive(gen)
}

func (c *Controller) publish(m Msg) {
	if c.bus != nil {
		c.bus.Publish(m)
	}
}

func (c *Controller) runLoop(ctx context.Context, gen int, prompt string, pendingSkills []string) {
	if !c.waitOrDone(ctx, gen, 120*time.Millisecond) {
		return
	}
	c.publish(SetActivityMsg{Activity: ActivityStreaming})

	if c.engine == nil {
		errText := "agent not configured"
		if c.engineErr != nil {
			errText = c.engineErr.Error()
		}
		if !c.Alive(gen) {
			return
		}
		c.publish(SessionEventMsg{Event: session.AssistantMessageUpdate{Message: session.Message{
			ID:    fmt.Sprintf("assistant-error-%d", time.Now().UnixNano()),
			State: session.StateError,
			Text:  errText,
			Content: []session.ContentBlock{
				{Type: session.BlockText, Text: errText},
			},
		}}})
		return
	}

	for ev, err := range c.engine.Loop(ctx, prompt, agent.LoopOpts{PendingSkills: pendingSkills}) {
		if !c.Alive(gen) {
			return
		}
		if err != nil {
			errText := err.Error()
			c.publish(SessionEventMsg{Event: session.AssistantMessageUpdate{Message: session.Message{
				ID:    fmt.Sprintf("assistant-error-%d", time.Now().UnixNano()),
				State: session.StateError,
				Text:  errText,
				Content: []session.ContentBlock{
					{Type: session.BlockText, Text: errText},
				},
			}}})
			return
		}
		if ev != nil {
			c.publish(SessionEventMsg{Event: ev})
		}
	}
}
