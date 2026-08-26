package clipboard

import (
	"errors"
	"runtime"
	"strings"
)

// CopyText writes plain text to the system clipboard.
func CopyText(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrEmpty
	}
	switch runtime.GOOS {
	case "darwin":
		return pipeToCommand("pbcopy", []byte(text))
	case "windows":
		if err := pipeToCommand("clip", []byte(text)); err == nil {
			return nil
		}
		return pipeToCommand("powershell", []byte(text), "-NoProfile", "-Command", "$Input | Set-Clipboard")
	default:
		if lookPath("wl-copy") {
			if err := pipeToCommand("wl-copy", []byte(text)); err == nil {
				return nil
			}
		}
		if lookPath("xclip") {
			return pipeToCommand("xclip", []byte(text), "-selection", "clipboard")
		}
		return errors.New("clipboard: copy text: install wl-clipboard or xclip")
	}
}
