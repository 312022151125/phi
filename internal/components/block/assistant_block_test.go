package block

import (
	"testing"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/stretchr/testify/assert"
)

func TestAssistantBlockDraw(t *testing.T) {
	tests := []struct {
		name     string
		ctx      components.DrawContext
		expected components.Surface
	}{
		{name: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assistantBlock := AssistantBlock{}
			surface := assistantBlock.Draw(tt.ctx)
			assert.Equal(t, tt.expected, surface)
		})
	}
}
