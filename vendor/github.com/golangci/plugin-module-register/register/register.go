package register

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	"golang.org/x/tools/go/analysis"
)

// Plugins load mode.
const (
	LoadModeSyntax    = "syntax"
	LoadModeTypesInfo = "typesinfo"
)

var (
	pluginsMu sync.RWMutex
	plugins   = make(map[string]NewPlugin)
)

// LinterPlugin the interface of the plugin structure.
type LinterPlugin interface {
	BuildAnalyzers() ([]*analysis.Analyzer, error)
	GetLoadMode() string
}

// NewPlugin the contract of the constructor of a plugin.
type NewPlugin func(conf any) (LinterPlugin, error)

// Plugin registers a plugin.
func Plugin(name string, p NewPlugin) {
	pluginsMu.Lock()

	plugins[name] = p

	pluginsMu.Unlock()
}

// GetPlugin gets a plugin by name.
func GetPlugin(name string) (NewPlugin, error) {
	pluginsMu.Lock()
	defer pluginsMu.Unlock()

	p, ok := plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found", name)
	}

	return p, nil
}

// DecodeSettings decode settings from golangci-lint to the structure of the plugin configuration.
func DecodeSettings[T any](rawSettings any) (T, error) {
	var buffer bytes.Buffer

	if err := json.NewEncoder(&buffer).Encode(rawSettings); err != nil {
		var zero T
		return zero, fmt.Errorf("encoding settings: %w", err)
	}

	decoder := json.NewDecoder(&buffer)
	decoder.DisallowUnknownFields()

	s := new(T)
	if err := decoder.Decode(s); err != nil {
		var zero T
		return zero, fmt.Errorf("decoding settings: %w", err)
	}

	return *s, nil
}
