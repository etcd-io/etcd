package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"

	jsonsch "github.com/golangci/golangci-lint/v2/jsonschema"
	"github.com/golangci/golangci-lint/v2/pkg/exitcodes"
)

func (c *configCommand) executeVerify(cmd *cobra.Command, _ []string) error {
	usedConfigFile := c.getUsedConfig()
	if usedConfigFile == "" {
		c.log.Warnf("No config file detected")
		os.Exit(exitcodes.NoConfigFileDetected)
	}

	c.log.Infof("Verifying the configuration file %q with the JSON Schema", usedConfigFile)

	err := validateConfiguration(jsonsch.NextSchema, usedConfigFile)
	if err != nil {
		var v *jsonschema.ValidationError
		if !errors.As(err, &v) {
			return fmt.Errorf("[%s] validate: %w", usedConfigFile, err)
		}

		printValidationDetail(cmd, v.DetailedOutput())

		return errors.New("the configuration contains invalid elements")
	}

	return nil
}

func validateConfiguration(schemaPath, targetFile string) error {
	compiler := jsonschema.NewCompiler()
	compiler.UseLoader(jsonsch.NewEmbedLoader())
	compiler.DefaultDraft(jsonschema.Draft7)

	// The name is not us
	schema, err := compiler.Compile(schemaPath)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}

	var m any

	switch strings.ToLower(filepath.Ext(targetFile)) {
	case ".yaml", ".yml", ".json":
		m, err = decodeYamlFile(targetFile)
		if err != nil {
			return err
		}

	case ".toml":
		m, err = decodeTomlFile(targetFile)
		if err != nil {
			return err
		}

	default:
		// unsupported
		return errors.New("unsupported configuration format")
	}

	return schema.Validate(m)
}

func printValidationDetail(cmd *cobra.Command, detail *jsonschema.OutputUnit) {
	if detail.Error != nil {
		data, _ := json.Marshal(detail.Error)
		details, _ := strconv.Unquote(string(data))

		cmd.PrintErrf("jsonschema: %q does not validate with %q: %s\n",
			strings.ReplaceAll(strings.TrimPrefix(detail.InstanceLocation, "/"), "/", "."), detail.KeywordLocation, details)
	}

	for _, d := range detail.Errors {
		printValidationDetail(cmd, &d)
	}
}

func decodeYamlFile(filename string) (any, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("[%s] file open: %w", filename, err)
	}

	defer func() { _ = file.Close() }()

	var m any
	err = yaml.NewDecoder(file).Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("[%s] YAML decode: %w", filename, err)
	}

	return m, nil
}

func decodeTomlFile(filename string) (any, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("[%s] file open: %w", filename, err)
	}

	defer func() { _ = file.Close() }()

	var m any
	err = toml.NewDecoder(file).Decode(&m)
	if err != nil {
		return nil, fmt.Errorf("[%s] TOML decode: %w", filename, err)
	}

	return m, nil
}
