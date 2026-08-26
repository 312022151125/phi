package chat

import (
	"strings"
	"testing"
)

func TestActiveQuestion(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		cursor int
		ok     bool
		query  string
	}{
		{"empty", "", 0, false, ""},
		{"question alone", "?", 1, true, ""},
		{"prefix", "?ctrl", 5, true, "ctrl"},
		{"cursor at start", "?help", 0, false, ""},
		{"after space", "?help me", 8, false, ""},
		{"not at start", "hi?", 3, false, ""},
		{"slash", "/sessions", 9, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, start, end, ok := ActiveQuestion(tt.value, tt.cursor)
			if ok != tt.ok {
				t.Fatalf("ok=%v want %v", ok, tt.ok)
			}
			if !ok {
				return
			}
			if q != tt.query {
				t.Fatalf("query=%q want %q", q, tt.query)
			}
			if start != 0 || end != tt.cursor {
				t.Fatalf("range=[%d,%d) want [0,%d)", start, end, tt.cursor)
			}
			if !strings.HasPrefix(tt.value, "?") {
				t.Fatal("expected ? prefix")
			}
		})
	}
}
