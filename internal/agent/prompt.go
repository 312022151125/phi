package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulseaiclub/phi/internal/llm/skills"
)

const defaultSystemPrompt = `You are phi, a coding agent in the terminal.

You help the user inspect and modify their local codebase. Prefer using tools
when you need real file contents or command output — do not invent paths or
command results.

## context
- your workspace: %s 
- your current dir: %s

## editing
- Always refresh LINE#HASH anchors immediately before every edit call: read the
target file in the same turn (read preferred; grep when locating lines). Never
run parallel edits on the same file. On anchor failure, re-read and retry once
with fresh anchors.

Keep replies concise. When you are done, answer without further tool calls.`

const defaultMaxToolRounds = 64

func GetCurrentDir() string {
	path, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return path
}

func GetWorkspaceDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// Prompt builds the system prompt. If skillPath is non-empty, skills are loaded
// from that directory and injected into the prompt.
func Prompt(skillPath string) string {
	base := fmt.Sprintf(defaultSystemPrompt, GetCurrentDir(), GetWorkspaceDir())
	skillBlock := loadSkillsBlock(skillPath)
	if skillBlock == "" {
		return base
	}
	return base + "\n\n# Skills\n\n" + skillBlock
}

// loadSkillsBlock loads skills from the given directory and returns a
// Markdown-formatted skills block. Returns "" if no skills are found.
func loadSkillsBlock(skillDir string) string {
	if skillDir == "" {
		return ""
	}
	list, err := skills.LoadSkills(skillDir)
	if err != nil || len(list) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Skills are reusable workflows loaded from the agent's skill directories.\n")
	sb.WriteString("Treat skills as constraints and shortcuts—apply the smallest relevant part.\n")
	sb.WriteString("\n")
	sb.WriteString("## How to use skills\n")
	sb.WriteString("- When a task matches a skill, read that skill's SKILL.md first and follow it.\n")
	sb.WriteString("- Interpret script/asset paths relative to that skill's directory.\n")
	sb.WriteString("- Follow each skill's name, description, and guidance; do not invent a parallel workflow.\n")
	sb.WriteString("\n")
	sb.WriteString("## Available skills\n")
	sb.WriteString(skills.ToPromptMarkdown(list))
	return sb.String()
}
