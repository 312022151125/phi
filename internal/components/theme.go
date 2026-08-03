package components

import (
	"strings"

	"github.com/pulseaiclub/xui"
)

// Theme holds semantic colors for transcript chrome.
type Theme struct {
	Foreground  xui.Style
	Muted       xui.Style
	Success     xui.Style
	Accent      xui.Style // links / "Show more"
	Warning     xui.Style // inline highlights / palette title
	Destructive xui.Style
	Border      xui.Style
	ToolName    xui.Style
	// Command palette.
	SelectionBg xui.Style // yellow bar behind selected row
	SelectionFg xui.Style // black text on selection
	Keybind     xui.Style // shortcut hints (Ctrl g)
	Command     xui.Style // command accent
}

// DefaultTheme returns the default dark RGB palette.
func DefaultTheme() Theme { return DarkTheme() }

// DarkTheme is the fixed RGB dark palette ("Dark").
func DarkTheme() Theme {
	return Theme{
		Foreground:  xui.Style{Fg: xui.DefaultColor()},
		Muted:       xui.Style{Fg: xui.IndexedColor(245), Dim: true},
		Success:     xui.Style{Fg: xui.RGBColor(0x7d, 0xc3, 0xa0), Bold: true},
		Accent:      xui.Style{Fg: xui.RGBColor(0xc4, 0x8a, 0xd9), Underline: true},
		Warning:     xui.Style{Fg: xui.RGBColor(0xe5, 0xc0, 0x7b)},
		Destructive: xui.Style{Fg: xui.RGBColor(0xe0, 0x6c, 0x75)},
		Border:      xui.Style{Fg: xui.IndexedColor(240)},
		ToolName:    xui.Style{Fg: xui.RGBColor(0x7d, 0xc3, 0xff)},
		SelectionBg: xui.Style{Bg: xui.RGBColor(0xe5, 0xc0, 0x7b)},
		SelectionFg: xui.Style{Fg: xui.RGBColor(0x00, 0x00, 0x00), Bold: true},
		Keybind:     xui.Style{Fg: xui.RGBColor(0x61, 0xaf, 0xef), Bold: true},
		Command:     xui.Style{Fg: xui.RGBColor(0xe5, 0xc0, 0x7b)},
	}
}

// TerminalTheme follows the terminal ANSI / default colors ("Terminal").
func TerminalTheme() Theme {
	return Theme{
		Foreground:  xui.Style{Fg: xui.DefaultColor()},
		Muted:       xui.Style{Fg: xui.IndexedColor(8), Dim: true},
		Success:     xui.Style{Fg: xui.IndexedColor(2), Bold: true},
		Accent:      xui.Style{Fg: xui.IndexedColor(5), Underline: true},
		Warning:     xui.Style{Fg: xui.IndexedColor(3)},
		Destructive: xui.Style{Fg: xui.IndexedColor(1)},
		Border:      xui.Style{Fg: xui.IndexedColor(8)},
		ToolName:    xui.Style{Fg: xui.IndexedColor(4)},
		SelectionBg: xui.Style{Bg: xui.IndexedColor(3)},
		SelectionFg: xui.Style{Fg: xui.IndexedColor(0), Bold: true},
		Keybind:     xui.Style{Fg: xui.IndexedColor(4), Bold: true},
		Command:     xui.Style{Fg: xui.IndexedColor(3)},
	}
}

// ThemeByName resolves a theme by display name (case-insensitive).
func ThemeByName(name string) (Theme, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "dark":
		return DarkTheme(), true
	case "terminal":
		return TerminalTheme(), true
	default:
		return Theme{}, false
	}
}
