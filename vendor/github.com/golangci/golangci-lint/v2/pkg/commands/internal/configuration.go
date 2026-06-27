package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const base = ".custom-gcl"

const defaultBinaryName = "custom-gcl"

// Configuration represents the configuration file.
type Configuration struct {
	// golangci-lint version.
	Version string `yaml:"version"`

	// Name of the binary.
	Name string `yaml:"name,omitempty"`

	// Destination is the path to a directory to store the binary.
	Destination string `yaml:"destination,omitempty"`

	// Plugins information.
	Plugins []*Plugin `yaml:"plugins,omitempty"`
}

// Validate checks and clean the configuration.
func (c *Configuration) Validate() error {
	if strings.TrimSpace(c.Version) == "" {
		return errors.New("root field 'version' is required")
	}

	if strings.TrimSpace(c.Name) == "" {
		c.Name = defaultBinaryName
	}

	if len(c.Plugins) == 0 {
		return errors.New("no plugins defined")
	}

	for _, plugin := range c.Plugins {
		if strings.TrimSpace(plugin.Module) == "" {
			return errors.New("field 'module' is required")
		}

		if strings.TrimSpace(plugin.Import) == "" {
			plugin.Import = plugin.Module
		}

		if strings.TrimSpace(plugin.Path) == "" && strings.TrimSpace(plugin.Version) == "" {
			return errors.New("missing information: 'version' or 'path' should be provided")
		}

		if strings.TrimSpace(plugin.Path) != "" && strings.TrimSpace(plugin.Version) != "" {
			return errors.New("invalid configuration: 'version' and 'path' should not be provided at the same time")
		}

		if strings.TrimSpace(plugin.Path) == "" {
			continue
		}

		abs, err := filepath.Abs(plugin.Path)
		if err != nil {
			return err
		}

		plugin.Path = abs
	}

	return nil
}

// Plugin represents information about a plugin.
type Plugin struct {
	// Module name.
	Module string `yaml:"module"`

	// Import to use.
	Import string `yaml:"import,omitempty"`

	// Version of the module.
	// Only for module available through a Go proxy.
	Version string `yaml:"version,omitempty"`

	// Path to the local module.
	// Only for local module.
	Path string `yaml:"path,omitempty"`
}

func LoadConfiguration() (*Configuration, error) {
	configFilePath, err := findConfigurationFile()
	if err != nil {
		return nil, fmt.Errorf("file %s not found: %w", configFilePath, err)
	}

	file, err := os.Open(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("file %s open: %w", configFilePath, err)
	}

	defer func() { _ = file.Close() }()

	var cfg Configuration

	err = yaml.NewDecoder(file).Decode(&cfg)
	if err != nil {
		return nil, fmt.Errorf("YAML decoding: %w", err)
	}

	return &cfg, nil
}

func findConfigurationFile() (string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("read directory: %w", err)
	}

	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())

		switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
		case "yml", "yaml", "json":
			if isConf(ext, entry.Name()) {
				return entry.Name(), nil
			}
		}
	}

	return "", errors.New("configuration file not found")
}

func isConf(ext, name string) bool {
	return base+ext == name
}
