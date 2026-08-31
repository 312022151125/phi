// Command hello is a minimal PXB extension.
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pulseaiclub/phi/ext"
	"github.com/pulseaiclub/phi/ext/sdk"
)

func main() {
	m := sdk.New("hello", "0.1.0")
	m.RegisterCommand("hello", ext.Command{
		Description: "Say hello via extension",
		Handler: func(args string, _ *ext.Context) error {
			msg := "Hello from PXB!"
			if args != "" {
				msg = fmt.Sprintf("Hello, %s!", args)
			}
			m.Notify("info", msg)
			return nil
		},
	})
	m.RegisterTool(ext.Tool{
		Name:        "ext_hello",
		Description: "Greet someone by name (extension tool)",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
		Execute: func(_ context.Context, args json.RawMessage) (ext.ToolResult, error) {
			var in struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(args, &in)
			return ext.ToolResult{Content: "Hello, " + in.Name + "!"}, nil
		},
	})
	if err := m.Run(); err != nil {
		panic(err)
	}
}
