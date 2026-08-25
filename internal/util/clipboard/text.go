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

// ReadText returns plain text from the system clipboard.
func ReadText() (string, error) {
	var (
		data []byte
		err  error
	)
	switch runtime.GOOS {
	case "darwin":
		data, err = runCommandTimeout(defaultReadTimeout, "pbpaste")
	case "windows":
		data, err = runCommandTimeout(
			defaultReadTimeout, "powershell", "-NoProfile", "-Command", "(Get-Clipboard -Raw).ToString()",
		)
	default:
		if isWaylandSession() && lookPath("wl-paste") {
			data, err = runCommandTimeout(defaultReadTimeout, "wl-paste", "--no-newline")
		} else if lookPath("xclip") {
			data, err = runCommandTimeout(defaultReadTimeout, "xclip", "-selection", "clipboard", "-o")
		} else if lookPath("wl-paste") {
			data, err = runCommandTimeout(defaultReadTimeout, "wl-paste", "--no-newline")
		} else {
			return "", errors.New("clipboard: read text: install wl-clipboard or xclip")
		}
	}
	if err != nil {
		return "", err
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return "", ErrEmpty
	}
	return text, nil
}
