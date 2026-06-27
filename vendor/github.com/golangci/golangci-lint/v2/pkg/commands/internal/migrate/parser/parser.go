package parser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"
)

type File interface {
	io.ReadWriter
	Name() string
}

// Decode decodes a file into data.
// The choice of the decoder is based on the file extension.
func Decode(file File, data any) error {
	ext := filepath.Ext(file.Name())

	switch strings.ToLower(ext) {
	case ".yaml", ".yml", ".json":
		err := yaml.NewDecoder(file).Decode(data)
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("YAML decode file %s: %w", file.Name(), err)
		}

	case ".toml":
		err := toml.NewDecoder(file).Decode(&data)
		if err != nil {
			return fmt.Errorf("TOML decode file %s: %w", file.Name(), err)
		}

	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}

	return nil
}

// Encode encodes data into a file.
// The choice of the encoder is based on the file extension.
func Encode(data any, dstFile File) error {
	ext := filepath.Ext(dstFile.Name())

	switch strings.ToLower(ext) {
	case ".yml", ".yaml":
		encoder := yaml.NewEncoder(dstFile)
		encoder.SetIndent(2)

		return encoder.Encode(data)

	case ".toml":
		encoder := toml.NewEncoder(dstFile)

		return encoder.Encode(data)

	case ".json":
		// The JSON encoder converts empty struct to `{}` instead of nothing (even with omitempty JSON struct tags).
		// So we need to use the YAML encoder as bridge to create JSON file.

		var buf bytes.Buffer
		err := yaml.NewEncoder(&buf).Encode(data)
		if err != nil {
			return err
		}

		raw := map[string]any{}
		err = yaml.NewDecoder(&buf).Decode(raw)
		if err != nil {
			return err
		}

		encoder := json.NewEncoder(dstFile)
		encoder.SetIndent("", "  ")

		return encoder.Encode(raw)

	default:
		return fmt.Errorf("unsupported file type: %s", ext)
	}
}
