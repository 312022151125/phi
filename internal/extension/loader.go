package extension

import (
	"fmt"
	"os"
	"reflect"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"

	"github.com/pulseaiclub/phi/ext"
)

// LoadFile evaluates a single extension .go file and invokes Extension(phi).
func LoadFile(path string, api *ext.API) error {
	if api == nil {
		return fmt.Errorf("extension: nil API")
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("extension: read %s: %w", path, err)
	}

	i := interp.New(interp.Options{GoPath: ""})
	if err := i.Use(stdlib.Symbols); err != nil {
		return fmt.Errorf("extension: stdlib symbols: %w", err)
	}
	if err := i.Use(extSymbols()); err != nil {
		return fmt.Errorf("extension: ext symbols: %w", err)
	}

	if _, err := i.Eval(string(src)); err != nil {
		return fmt.Errorf("extension: eval %s: %w", path, err)
	}

	v, err := lookupExtension(i)
	if err != nil {
		return fmt.Errorf("extension: %s: %w", path, err)
	}

	fn, ok := v.Interface().(func(*ext.API))
	if !ok {
		// yaegi may wrap as func(*ext.API) under a different concrete type; try Call.
		if v.Kind() != reflect.Func {
			return fmt.Errorf("extension: Extension is not a function")
		}
		results := v.Call([]reflect.Value{reflect.ValueOf(api)})
		_ = results
		return nil
	}
	fn(api)
	return nil
}

func lookupExtension(i *interp.Interpreter) (reflect.Value, error) {
	// Prefer package-qualified lookup; fall back to bare name for package main.
	candidates := []string{
		"main.Extension",
		"Extension",
	}
	var lastErr error
	for _, name := range candidates {
		v, err := i.Eval(name)
		if err == nil && v.IsValid() {
			if v.Kind() == reflect.Func || (v.Kind() == reflect.Interface && !v.IsNil()) {
				return v, nil
			}
			if v.Kind() != reflect.Invalid && !isNilValue(v) {
				return v, nil
			}
		}
		lastErr = err
	}
	if lastErr != nil {
		return reflect.Value{}, fmt.Errorf(
			"Extension symbol not found (export func Extension in package main): %w",
			lastErr,
		)
	}
	return reflect.Value{}, fmt.Errorf("Extension symbol not found (export func Extension in package main)")
}

func isNilValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
