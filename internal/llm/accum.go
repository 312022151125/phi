package llm

import (
	"strings"
)

type streamAccumulator struct {
	role      Role
	content   strings.Builder
	reasoning strings.Builder
	toolCalls map[int]*toolCallAcc
	maxIndex  int
}

type toolCallAcc struct {
	id   string
	typ  string
	name string
	args strings.Builder
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{
		toolCalls: make(map[int]*toolCallAcc),
		maxIndex:  -1,
	}
}

func (s *streamAccumulator) applyDelta(delta StreamDelta) {
	if delta.Role != "" {
		s.role = Role(delta.Role)
	}
	if delta.Content != "" {
		s.content.WriteString(delta.Content)
	}
	if delta.ReasoningContent != "" {
		s.reasoning.WriteString(delta.ReasoningContent)
	}
	for _, tc := range delta.ToolCalls {
		s.applyToolCallDelta(tc)
	}
}

func (s *streamAccumulator) applyMessage(msg *Message) {
	if msg == nil {
		return
	}
	if strings.TrimSpace(msg.Content) != "" {
		s.content.Reset()
		s.content.WriteString(msg.Content)
	}
	if strings.TrimSpace(msg.ReasoningContent) != "" {
		s.reasoning.Reset()
		s.reasoning.WriteString(msg.ReasoningContent)
	}
	for _, tc := range msg.ToolCalls {
		s.applyToolCallDelta(tc)
	}
}

func (s *streamAccumulator) applyToolCallDelta(tc ToolCall) {
	if tc.Index > s.maxIndex {
		s.maxIndex = tc.Index
	}
	acc := s.toolCalls[tc.Index]
	if acc == nil {
		acc = &toolCallAcc{typ: "function"}
		s.toolCalls[tc.Index] = acc
	}
	if acc.id == "" && tc.ID != "" {
		acc.id = tc.ID
	}
	if tc.Type != "" {
		acc.typ = tc.Type
	}
	if tc.Function.Name != "" {
		acc.name = tc.Function.Name
	}
	if tc.Function.Arguments != "" {
		acc.args.WriteString(tc.Function.Arguments)
	}
}

func (s *streamAccumulator) message() Message {
	role := s.role
	if role == "" {
		role = RoleAssistant
	}
	return Message{
		Role:             role,
		Content:          s.content.String(),
		ReasoningContent: s.reasoning.String(),
		ToolCalls:        s.orderedToolCalls(),
	}
}

func (s *streamAccumulator) orderedToolCalls() []ToolCall {
	if s.maxIndex < 0 {
		return nil
	}
	out := make([]ToolCall, 0, s.maxIndex+1)
	for i := 0; i <= s.maxIndex; i++ {
		acc, ok := s.toolCalls[i]
		if !ok {
			continue
		}
		out = append(out, ToolCall{
			Index: i,
			ID:    acc.id,
			Type:  acc.typ,
			Function: Function{
				Name:      acc.name,
				Arguments: acc.args.String(),
			},
		})
	}
	return out
}
