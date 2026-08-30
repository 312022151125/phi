package extension

import (
	"github.com/pulseaiclub/phi/ext"
	"github.com/pulseaiclub/phi/internal/debuglog"
)

// Load discovers and evaluates extensions from user and project dirs.
func Load(userDir, projectDir string) (*Runner, []Warning, error) {
	found, warns, err := Discover(userDir, projectDir)
	if err != nil {
		return nil, warns, err
	}
	r := &Runner{}
	for _, d := range found {
		api := ext.NewAPI()
		if err := LoadFile(d.Path, api); err != nil {
			warns = append(warns, Warning{Path: d.Path, Message: err.Error()})
			debuglog.Logf("extension: load %s: %v", d.Path, err)
			continue
		}
		r.apis = append(r.apis, api)
		r.loaded = append(r.loaded, d)
	}
	r.warns = warns
	return r, warns, nil
}
