package config

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

// FromYAML parses configuration from given YAML content.
func FromYAML(in []byte) (*PlainConfig, error) {
	raw := PlainConfig{}
	if err := yaml.Unmarshal(in, &raw); err != nil {
		return nil, fmt.Errorf("cannot parse config from YAML file: %w", err)
	}

	if raw.Version != nil && !strings.HasPrefix(*raw.Version, "1.") {
		return nil, fmt.Errorf("unsupported config version: %s", *raw.Version)
	}

	return &raw, nil
}

// FromYAMLFile parses configuration from given file path.
func FromYAMLFile(path string) (*PlainConfig, error) {
	in, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read file (%s): %w", path, err)
	}

	raw := PlainConfig{}
	if err := yaml.Unmarshal(in, &raw); err != nil {
		return nil, fmt.Errorf("cannot parse config from YAML file: %w", err)
	}

	if raw.Version != nil && !strings.HasPrefix(*raw.Version, "1.") {
		return nil, fmt.Errorf("unsupported config version: %s", *raw.Version)
	}

	return &raw, nil
}
