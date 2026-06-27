// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/command"
	"github.com/google/yamlfmt/engine"
	"github.com/google/yamlfmt/formatters/kyaml"
	"github.com/google/yamlfmt/internal/collections"
	"github.com/google/yamlfmt/internal/logger"
	"github.com/google/yamlfmt/pkg/yaml"
	"github.com/mitchellh/mapstructure"
)

var configFileNames = collections.Set[string]{
	".yamlfmt":      {},
	".yamlfmt.yml":  {},
	".yamlfmt.yaml": {},
	"yamlfmt.yml":   {},
	"yamlfmt.yaml":  {},
}

const configHomeDir string = "yamlfmt"

var (
	errNoConfFlag       = errors.New("config path not specified in --conf")
	errConfPathInvalid  = errors.New("config path specified in --conf was invalid")
	errConfPathNotExist = errors.New("no config file found")
	errConfPathIsDir    = errors.New("config path is dir")
	errNoConfigHome     = errors.New("missing required env var for config home")
)

type configPathError struct {
	path string
	err  error
}

func (e *configPathError) Error() string {
	if errors.Is(e.err, errConfPathInvalid) {
		return fmt.Sprintf("config path %s was invalid", e.path)
	}
	if errors.Is(e.err, errConfPathNotExist) {
		return fmt.Sprintf("no config file found in directory %s", filepath.Dir(e.path))
	}
	if errors.Is(e.err, errConfPathIsDir) {
		return fmt.Sprintf("config path %s is a directory", e.path)
	}
	return e.err.Error()
}

func (e *configPathError) Unwrap() error {
	return e.err
}

func readConfig(path string) (map[string]any, error) {
	yamlBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var configData map[string]interface{}
	err = yaml.Unmarshal(yamlBytes, &configData)
	if err != nil {
		return nil, err
	}
	return configData, nil
}

func getConfigPath() (string, error) {
	// First priority: specified in cli flag
	configPath, err := getConfigPathFromFlag()
	if err != nil {
		// If they don't provide a conf flag, we continue. If
		// a conf flag is provided and it's wrong, we consider
		// that a failure state.
		if !errors.Is(err, errNoConfFlag) {
			return "", err
		}
	} else {
		return configPath, nil
	}

	// Second priority: in the working directory
	configPath, err = getConfigPathFromDirTree()
	// In this scenario, no errors are considered a failure state,
	// so we continue to the next fallback if there are no errors.
	if err == nil {
		return configPath, nil
	}

	if !*flagDisableGlobalConf {
		// Third priority: in home config directory
		configPath, err = getConfigPathFromConfigHome()
		// In this scenario, no errors are considered a failure state,
		// so we continue to the next fallback if there are no errors.
		if err == nil {
			return configPath, nil
		}
	}

	// All else fails, no path and no error (signals to
	// use default config).
	logger.Debug(logger.DebugCodeConfig, "No config file found, using default config")
	return "", nil
}

func getConfigPathFromFlag() (string, error) {
	// First check if the global configuration was explicitly requested as that takes precedence.
	if *flagGlobalConf {
		logger.Debug(logger.DebugCodeConfig, "Using -global_conf flag")
		return getConfigPathFromXdgConfigHome()
	}
	// If the global config wasn't explicitly requested, check if there was a specific configuration path supplied.
	configPath := *flagConf
	if configPath != "" {
		logger.Debug(logger.DebugCodeConfig, "Using config path %s from -conf flag", configPath)
		return configPath, validatePath(configPath)
	}

	logger.Debug(logger.DebugCodeConfig, "No config path specified in -conf")
	return configPath, errNoConfFlag
}

// This function searches up the directory tree until it finds
// a config file.
func getConfigPathFromDirTree() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(wd)
	if err != nil {
		return "", err
	}
	dir := absPath
	for dir != filepath.Dir(dir) {
		configPath, err := getConfigPathFromDir(dir)
		if err == nil {
			logger.Debug(logger.DebugCodeConfig, "Found config at %s", configPath)
			return configPath, nil
		}
		dir = filepath.Dir(dir)
	}
	return "", errConfPathNotExist
}

func getConfigPathFromConfigHome() (string, error) {
	// Build tags are a veritable pain in the behind,
	// I'm putting both config home functions in this
	// file. You can't stop me.
	if runtime.GOOS == "windows" {
		return getConfigPathFromAppDataLocal()
	}
	return getConfigPathFromXdgConfigHome()
}

func getConfigPathFromXdgConfigHome() (string, error) {
	configHome, configHomePresent := os.LookupEnv("XDG_CONFIG_HOME")
	if !configHomePresent {
		home, homePresent := os.LookupEnv("HOME")
		if !homePresent {
			// I fear whom's'tever does not have a $HOME set
			return "", errNoConfigHome
		}
		configHome = filepath.Join(home, ".config")
	}
	homeConfigPath := filepath.Join(configHome, configHomeDir)
	return getConfigPathFromDir(homeConfigPath)
}

func getConfigPathFromAppDataLocal() (string, error) {
	configHome, configHomePresent := os.LookupEnv("LOCALAPPDATA")
	if !configHomePresent {
		// I think you'd have to go out of your way to unset this,
		// so this should only happen to sickos with broken setups.
		return "", errNoConfigHome
	}
	homeConfigPath := filepath.Join(configHome, configHomeDir)
	return getConfigPathFromDir(homeConfigPath)
}

func getConfigPathFromDir(dir string) (string, error) {
	for filename := range configFileNames {
		configPath := filepath.Join(dir, filename)
		if err := validatePath(configPath); err == nil {
			logger.Debug(logger.DebugCodeConfig, "Found config at %s", configPath)
			return configPath, nil
		}
	}
	logger.Debug(logger.DebugCodeConfig, "No config file found in %s", dir)
	return "", errConfPathNotExist
}

func validatePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &configPathError{
				path: path,
				err:  errConfPathNotExist,
			}
		}
		if info.IsDir() {
			return &configPathError{
				path: path,
				err:  errConfPathIsDir,
			}
		}
		return &configPathError{
			path: path,
			err:  err,
		}
	}
	return nil
}

func makeCommandConfigFromData(configData map[string]any) (*command.Config, error) {
	altFormatter, err := detectAlternateFormatterFlag()
	if err != nil {
		return nil, err
	}

	config := command.Config{FormatterConfig: command.NewFormatterConfig()}
	if altFormatter != "" {
		// If an alternate formatter was specified via CLI flag, override
		// the formatter from the configuration.
		config.FormatterConfig.Type = altFormatter
	} else {
		err := mapstructure.Decode(configData, &config)
		if err != nil {
			return nil, err
		}
	}

	// Parse overrides for formatter configuration
	if len(flagFormatter) > 0 {
		overrides, err := parseFormatterConfigFlag(flagFormatter)
		if err != nil {
			return nil, err
		}
		for k, v := range overrides {
			if k == "type" {
				config.FormatterConfig.Type = v.(string)
			}
			config.FormatterConfig.FormatterSettings[k] = v
		}
	}

	// Default to OS line endings
	if config.LineEnding == "" {
		config.LineEnding = yamlfmt.LineBreakStyleLF
		if runtime.GOOS == "windows" {
			config.LineEnding = yamlfmt.LineBreakStyleCRLF
		}
	}

	// Default to yaml and yml extensions
	if len(config.Extensions) == 0 {
		config.Extensions = []string{"yaml", "yml"}
	}

	// Apply the general rule that the config takes precedence over
	// the command line flags.
	if !config.Doublestar {
		config.Doublestar = *flagDoublestar
	}
	if !config.ContinueOnError {
		config.ContinueOnError = *flagContinueOnError
	}
	if !config.GitignoreExcludes {
		config.GitignoreExcludes = *flagGitignoreExcludes
	}
	config.GitignorePath = pickFirst(config.GitignorePath, *flagGitignorePath)
	config.OutputFormat = pickFirst(config.OutputFormat, getOutputFormatFromFlag(), engine.EngineOutputDefault)

	defaultMatchType := yamlfmt.MatchTypeStandard
	if config.Doublestar {
		defaultMatchType = yamlfmt.MatchTypeDoublestar
	}
	config.MatchType = pickFirst(config.MatchType, yamlfmt.MatchType(*flagMatchType), defaultMatchType)

	// Overwrite config if includes are provided through args
	if len(flag.Args()) > 0 {
		config.Include = flag.Args()
	}

	// Append any additional data from array flags
	config.Exclude = append(config.Exclude, flagExclude...)
	config.Extensions = append(config.Extensions, flagExtensions...)

	return &config, nil
}

// pickFirst returns the first string in ss that is not empty.
func pickFirst[T ~string](ss ...T) T {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}

	return ""
}

func parseFormatterConfigFlag(flagValues []string) (map[string]any, error) {
	formatterValues := map[string]any{}
	flagErrors := collections.Errors{}

	// Expected format: fieldname=value
	for _, configField := range flagValues {
		if strings.Count(configField, "=") != 1 {
			flagErrors = append(
				flagErrors,
				fmt.Errorf("badly formatted config field: %s", configField),
			)
			continue
		}

		kv := strings.Split(configField, "=")

		// Try to parse as integer
		vInt, err := strconv.ParseInt(kv[1], 10, 64)
		if err == nil {
			formatterValues[kv[0]] = vInt
			continue
		}

		// Try to parse as boolean
		vBool, err := strconv.ParseBool(kv[1])
		if err == nil {
			formatterValues[kv[0]] = vBool
			continue
		}

		// Fall through to parsing as string
		formatterValues[kv[0]] = kv[1]
	}

	return formatterValues, flagErrors.Combine()
}

func detectAlternateFormatterFlag() (string, error) {
	// Right now there is only one alternate formatter flag so I'm
	// being a bit cheap with the implementation. If another formatter
	// is added, this will need to return an error if more than one
	// is requested.
	if *flagKyaml {
		return kyaml.KYAMLFormatterType, nil
	}
	return "", nil
}
