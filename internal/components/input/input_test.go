package input

import (
	"testing"

	"github.com/pulseaiclub/xui"

	"github.com/stretchr/testify/require"

	"github.com/pulseaiclub/phi/internal/components"
	"github.com/pulseaiclub/phi/internal/components/layout"
)

func TestTextFieldCtrlUClears(t *testing.T) {
	f := &TextField{Value: "ab", Cursor: 1}
	ctx := &components.EventContext{}
	changed := 0
	f.OnChange = func(string) { changed++ }
	f.Handle(ctx, xui.KeyEvent{Code: xui.KeyRune, Rune: 'u', Mods: xui.ModCtrl, Press: true})
	require.Empty(t, f.Value)
	require.Zero(t, f.Cursor)
	require.Equal(t, 1, changed)
	require.True(t, ctx.Consume)
}

func TestTextFieldCtrlUEmptyDoesNotNotify(t *testing.T) {
	f := &TextField{}
	ctx := &components.EventContext{}
	changed := 0
	f.OnChange = func(string) { changed++ }
	f.Handle(ctx, xui.KeyEvent{Code: xui.KeyRune, Rune: 'u', Mods: xui.ModCtrl, Press: true})
	require.Zero(t, changed)
	require.True(t, ctx.Consume)
}

func TestDiffBlock(t *testing.T) {
	d := &DiffBlock{Diff: "+added\n-removed\n context", Theme: components.DefaultTheme()}
	ds := d.Draw(components.DrawContext{Max: components.Size{Width: 40, Height: 10}})
	if ds.Size.Height != 3 {
		t.Fatalf("diff lines %d", ds.Size.Height)
	}
}

func TestModalMarkdown(t *testing.T) {
	md := &Markdown{Source: "# Hello\n- item `code`", Theme: components.DefaultTheme()}
	ms := md.Draw(components.DrawContext{Max: components.Size{Width: 40, Height: 10}})
	if ms.Size.Height < 2 {
		t.Fatalf("markdown h=%d", ms.Size.Height)
	}
	modal := &Modal{
		Title:  "Confirm",
		Body:   &layout.Text{Content: "Sure?"},
		Footer: "Esc close",
		Width:  40,
		Theme:  components.DefaultTheme(),
	}
	s := modal.Draw(components.DrawContext{Max: components.Size{Width: 80, Height: 24}})
	if len(s.Children) != 1 {
		t.Fatalf("modal children %d", len(s.Children))
	}
}
