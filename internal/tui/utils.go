package tui

import (
	"os"
	"path/filepath"
	"strings"
)

func shortPath(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(p, home) {
		p = "~" + p[len(home):]
	}
	parts := strings.Split(p, string(filepath.Separator))
	if len(parts) <= 5 {
		return p
	}
	return strings.Join(parts[:2], string(filepath.Separator)) +
		string(filepath.Separator) + ".." + string(filepath.Separator) +
		strings.Join(parts[len(parts)-2:], string(filepath.Separator))
}
