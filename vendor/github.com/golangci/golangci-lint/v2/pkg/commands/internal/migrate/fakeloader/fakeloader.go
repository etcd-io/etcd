package fakeloader

import (
	"fmt"
	"os"

	"github.com/go-viper/mapstructure/v2"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/parser"
	"github.com/golangci/golangci-lint/v2/pkg/config"
)

// Load is used to keep case of configuration.
// Viper serialize raw map keys in lowercase, this is a problem with the configuration of some linters.
func Load(srcPath string, old any) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	defer func() { _ = file.Close() }()

	raw := map[string]any{}

	err = parser.Decode(file, raw)
	if err != nil {
		return err
	}

	// NOTE: this is inspired by viper internals.
	cc := &mapstructure.DecoderConfig{
		Result:           old,
		WeaklyTypedInput: true,
		DecodeHook:       config.DecodeHookFunc(),
	}

	decoder, err := mapstructure.NewDecoder(cc)
	if err != nil {
		return fmt.Errorf("constructing mapstructure decoder: %w", err)
	}

	err = decoder.Decode(raw)
	if err != nil {
		return fmt.Errorf("decoding configuration file: %w", err)
	}

	return nil
}
