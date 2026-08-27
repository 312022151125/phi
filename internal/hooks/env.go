package hooks

import (
	"strings"
)

// Env keys injected into every command hook process.
const (
	EnvHookEvent  = "PHI_HOOK_EVENT"
	EnvSessionID  = "PHI_SESSION_ID"
	EnvCwd        = "PHI_CWD"
	EnvProjectDir = "PHI_PROJECT_DIR"
)

var sensitiveEnvSubstrings = []string{
	"API_KEY",
	"SECRET",
	"TOKEN",
	"PASSWORD",
	"CREDENTIAL",
	"PRIVATE_KEY",
	"AUTHORIZATION",
	"BEARER",
	"OAUTH",
	"AWS_ACCESS_KEY",
	"AWS_SECRET",
	"AWS_SESSION_TOKEN",
	"PHI_API_KEY",
}

type hookEnv struct {
	Event      string
	SessionID  string
	Cwd        string
	ProjectDir string
}

func sanitizeEnv(parent []string, extra hookEnv) []string {
	out := make([]string, 0, len(parent)+4)
	for _, kv := range parent {
		key, _, _ := strings.Cut(kv, "=")
		if isSensitiveEnvKey(key) {
			continue
		}
		switch key {
		case EnvHookEvent, EnvSessionID, EnvCwd, EnvProjectDir:
			continue
		}
		out = append(out, kv)
	}
	out = append(out,
		EnvHookEvent+"="+extra.Event,
		EnvSessionID+"="+extra.SessionID,
		EnvCwd+"="+extra.Cwd,
		EnvProjectDir+"="+extra.ProjectDir,
	)
	return out
}

func isSensitiveEnvKey(key string) bool {
	k := strings.ToUpper(key)
	for _, sub := range sensitiveEnvSubstrings {
		if strings.Contains(k, sub) {
			return true
		}
	}
	return false
}
