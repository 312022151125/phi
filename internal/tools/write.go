package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulseaiclub/phi/internal/llm"
)

var writeDescription = `Write content to a new file. Fails if the file already exists.
Pass the file path and the content string to write.`

// WriteTool returns the write tool definition + handler.
func WriteTool() Tool {
	return Tool{
		Definition: llm.ToolDefinition{
			Name:        "write",
			Description: writeDescription,
			Params: &llm.FunctionParameters{
				Type: "object",
				Properties: llm.Object{
					"path": llm.Object{
						"type":        "string",
						"description": "Path to a new file to create. Must not already exist. Example: src/new.go",
					},
					"content": llm.Object{
						"type":        "string",
						"description": "Content to write to the file.",
					},
				},
				Required: []string{"path", "content"},
			},
		},
		DetailFromArgs: func(input json.RawMessage) string {
			var in writeInput
			_ = json.Unmarshal(input, &in)
			return strings.TrimSpace(in.Path)
		},
		Run: runWrite,
	}
}

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func runWrite(ctx context.Context, input json.RawMessage) (Result, error) {
	var in writeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, fmt.Errorf("failed to parse write arguments: %w", err)
	}
	path := strings.TrimSpace(in.Path)
	if path == "" {
		return Result{}, fmt.Errorf("path is required")
	}
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return Result{}, err
		}
		path = filepath.Join(cwd, path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Result{}, fmt.Errorf("failed to create parent directories: %w", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return Result{}, fmt.Errorf("file already exists: %s", path)
		}
		return Result{}, fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(in.Content); err != nil {
		return Result{}, fmt.Errorf("failed to write content: %w", err)
	}

	detail := fmt.Sprintf("wrote %d bytes to %s", len(in.Content), path)
	return Result{Content: detail, Detail: path, Output: detail}, nil
}
