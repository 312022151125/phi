package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Source labels where a discovered extension came from.
const (
	SourceUser    = "user"
	SourceProject = "project"
)

// EnvExtensions disables extension loading when set to "off".
const EnvExtensions = "PHI_EXTENSIONS"

// Warning is a non-fatal discovery or load problem.
type Warning struct {
	Path    string
	Message string
}

func (w Warning) String() string {
	if w.Path == "" {
		return w.Message
	}
	return w.Path + ": " + w.Message
}

// Discovered is one extension directory with a phi.yaml manifest.
type Discovered struct {
	ID       string // directory name / manifest name
	Path     string // absolute path to extension directory
	Manifest Manifest
	Source   string
}

// ExtensionsDisabled reports whether PHI_EXTENSIONS=off.
func ExtensionsDisabled() bool {
	v := strings.TrimSpace(os.Getenv(EnvExtensions))
	return strings.EqualFold(v, "off")
}

// Discover finds extension directories under userDir then projectDir.
// Same ID: project replaces user. Layout:
//
//	<dir>/<id>/phi.yaml   (exec points at a PXB binary)
func Discover(userDir, projectDir string) ([]Discovered, []Warning, error) {
	if ExtensionsDisabled() {
		return nil, nil, nil
	}

	byID := make(map[string]Discovered)
	var warnings []Warning

	load := func(dir, source string) error {
		if dir == "" {
			return nil
		}
		found, warns, err := scanDir(dir, source)
		warnings = append(warnings, warns...)
		if err != nil {
			return err
		}
		for _, d := range found {
			byID[d.ID] = d
		}
		return nil
	}

	if err := load(userDir, SourceUser); err != nil {
		return nil, warnings, err
	}
	if err := load(projectDir, SourceProject); err != nil {
		return nil, warnings, err
	}

	out := make([]Discovered, 0, len(byID))
	for _, d := range byID {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, warnings, nil
}

func scanDir(dir, source string) ([]Discovered, []Warning, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("extension: read dir %s: %w", dir, err)
	}

	var (
		out      []Discovered
		warnings []Warning
		seen     = make(map[string]string)
	)

	for _, ent := range entries {
		name := ent.Name()
		if strings.HasPrefix(name, ".") || !ent.IsDir() {
			if !ent.IsDir() && (strings.HasSuffix(name, ".go") || name == "index.go") {
				warnings = append(warnings, Warning{
					Path: filepath.Join(dir, name),
					Message: "yaegi .go extensions are no longer loaded; " +
						"build a PXB binary and add phi.yaml (see doc/extensions.md)",
				})
			}
			continue
		}
		full := filepath.Join(dir, name)
		m, err := ReadManifest(full)
		if err != nil {
			if os.IsNotExist(err) {
				// Subdir without manifest: ignore unless it looks like a legacy go plugin.
				if _, e2 := dirEntryFile(full); e2 == nil {
					if entry, _ := dirEntryFile(full); entry != "" {
						warnings = append(warnings, Warning{
							Path: full,
							Message: "legacy Go source extension ignored; " +
								"migrate to PXB (phi.yaml + compiled binary)",
						})
					}
				}
				continue
			}
			warnings = append(warnings, Warning{Path: full, Message: err.Error()})
			continue
		}
		if !m.IsEnabled() {
			continue
		}
		id := m.Name
		if id == "" {
			id = name
		}
		if prev, dup := seen[id]; dup {
			warnings = append(warnings, Warning{
				Path:    full,
				Message: fmt.Sprintf("duplicate extension %q (already %s); skipped", id, prev),
			})
			continue
		}
		seen[id] = full
		out = append(out, Discovered{ID: id, Path: full, Manifest: m, Source: source})
	}
	return out, warnings, nil
}

// dirEntryFile returns a legacy yaegi entry (kept for migration warnings).
func dirEntryFile(dir string) (string, error) {
	index := filepath.Join(dir, "index.go")
	st, err := os.Stat(index)
	if err == nil && !st.IsDir() {
		return index, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return soleRootGoFile(dir)
}

func soleRootGoFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var found string
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if found != "" {
			return "", nil
		}
		found = filepath.Join(dir, name)
	}
	return found, nil
}

// FormatDiscovered returns a one-line status for palette / logs.
func FormatDiscovered(d Discovered) string {
	return fmt.Sprintf("%s  [%s]  %s", d.ID, d.Source, d.Path)
}
