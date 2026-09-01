package agent

import (
	"errors"

	"github.com/pulseaiclub/phi/internal/extension"
	"github.com/pulseaiclub/phi/internal/job"
	"github.com/pulseaiclub/phi/internal/llm"
)

// NewJobManager creates a process-level job manager whose runner drives child Engines.
// modelFn may be nil; then model is used as a fixed snapshot.
// extensionsFn supplies extensions for child engines (may return nil); prefer a live
// getter so TUI reload updates sub-agents too.
func NewJobManager(
	root string,
	model llm.ModelConfig,
	modelFn func() llm.ModelConfig,
	extensionsFn func() *extension.Runner,
) (*job.Manager, error) {
	return NewJobManagerWithModelResolver(root, model, modelFn, extensionsFn, nil)
}

// NewJobManagerWithModelResolver is NewJobManager plus an optional lookup for
// per-job model overrides. A nil resolver preserves the original single-model
// fast path exactly.
func NewJobManagerWithModelResolver(
	root string,
	model llm.ModelConfig,
	modelFn func() llm.ModelConfig,
	extensionsFn func() *extension.Runner,
	modelResolver func(string) (llm.ModelConfig, bool),
) (*job.Manager, error) {
	if root == "" {
		return nil, errors.New("agent: jobs root is required")
	}
	return job.New(job.Options{
		Root: root,
		Runner: EngineRunner{
			Model:         model,
			ModelFn:       modelFn,
			ModelResolver: modelResolver,
			ExtensionsFn:  extensionsFn,
		},
	})
}
