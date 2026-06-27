// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/engine"
	"github.com/google/yamlfmt/pkg/yaml"
	"github.com/mitchellh/mapstructure"
)

type FormatterConfig struct {
	Type              string         `mapstructure:"type"`
	FormatterSettings map[string]any `mapstructure:",remain"`
}

// NewFormatterConfig returns an empty formatter config with all fields initialized.
func NewFormatterConfig() *FormatterConfig {
	return &FormatterConfig{FormatterSettings: make(map[string]any)}
}

type Config struct {
	Extensions        []string                  `mapstructure:"extensions"`
	MatchType         yamlfmt.MatchType         `mapstructure:"match_type"`
	Include           []string                  `mapstructure:"include"`
	Exclude           []string                  `mapstructure:"exclude"`
	RegexExclude      []string                  `mapstructure:"regex_exclude"`
	FormatterConfig   *FormatterConfig          `mapstructure:"formatter,omitempty"`
	Doublestar        bool                      `mapstructure:"doublestar"`
	ContinueOnError   bool                      `mapstructure:"continue_on_error"`
	LineEnding        yamlfmt.LineBreakStyle    `mapstructure:"line_ending"`
	GitignoreExcludes bool                      `mapstructure:"gitignore_excludes"`
	GitignorePath     string                    `mapstructure:"gitignore_path"`
	OutputFormat      engine.EngineOutputFormat `mapstructure:"output_format"`
}

type Command struct {
	Operation yamlfmt.Operation
	Registry  *yamlfmt.Registry
	Config    *Config
	Quiet     bool
	Verbose   bool
}

func (c *Command) Run() error {
	formatter, err := c.getFormatter()
	if err != nil {
		return err
	}

	lineSepChar, err := c.Config.LineEnding.Separator()
	if err != nil {
		return err
	}

	eng := &engine.ConsecutiveEngine{
		LineSepCharacter: lineSepChar,
		Formatter:        formatter,
		Quiet:            c.Quiet,
		Verbose:          c.Verbose,
		ContinueOnError:  c.Config.ContinueOnError,
		OutputFormat:     c.Config.OutputFormat,
	}

	var paths []string
	// If the operation is stdin, skip path analysis. You can only
	// read from /dev/stdin once, so we don't want to read it
	// if that's the argument the user passed in.
	if c.Operation == yamlfmt.OperationStdin {
		paths = []string{}
	} else {
		collectedPaths, err := c.collectPaths()
		if err != nil {
			return err
		}
		if c.Config.GitignoreExcludes {
			newPaths, err := yamlfmt.ExcludeWithGitignore(c.Config.GitignorePath, collectedPaths)
			if err != nil {
				return err
			}
			collectedPaths = newPaths
		}
		paths, err = c.analyzePaths(collectedPaths)
		if err != nil {
			fmt.Printf("path analysis found the following errors:\n%v", err)
			fmt.Println("Continuing...")
		}
	}

	switch c.Operation {
	case yamlfmt.OperationFormat:
		out, err := eng.Format(paths)
		if out != nil {
			fmt.Print(out)
		}
		if err != nil {
			return err
		}
	case yamlfmt.OperationLint:
		out, err := eng.Lint(paths)
		if err != nil {
			return err
		}
		if out != nil {
			// This will be picked up by log.Fatal in main() and
			// cause an exit code of 1, which is a critical
			// component of the lint functionality.
			return errors.New(out.String())
		}
	case yamlfmt.OperationDry:
		out, err := eng.DryRun(paths)
		if err != nil {
			return err
		}
		if out != nil {
			fmt.Print(out)
		} else if !c.Quiet {
			fmt.Println("No files will be changed.")
		}
	case yamlfmt.OperationStdin:
		stdinYaml, err := readFromStdin()
		if err != nil {
			return err
		}
		out, err := eng.FormatContent(stdinYaml)
		if err != nil {
			return err
		}
		fmt.Printf("%s", out)
	case yamlfmt.OperationPrintConfig:
		commandConfig := map[string]any{}
		err = mapstructure.Decode(c.Config, &commandConfig)
		if err != nil {
			return err
		}
		delete(commandConfig, "formatter")
		out, err := yaml.Marshal(commandConfig)
		if err != nil {
			return err
		}
		fmt.Printf("%s", out)

		formatterConfigMap, err := formatter.ConfigMap()
		if err != nil {
			return err
		}
		out, err = yaml.Marshal(map[string]any{
			"formatter": formatterConfigMap,
		})
		if err != nil {
			return err
		}
		fmt.Printf("%s", out)
	}

	return nil
}

func (c *Command) getFormatter() (yamlfmt.Formatter, error) {
	var factoryType string

	// In the existing codepaths, this value is always set. But
	// it's a habit of mine to check anything that can possibly be nil
	// if I remember that to be the case. :)
	if c.Config.FormatterConfig != nil {
		factoryType = c.Config.FormatterConfig.Type

		// The line ending set within the formatter settings takes precedence over setting
		// it from the top level config. If it's not set in formatter settings, then
		// we use the value from the top level.
		if _, ok := c.Config.FormatterConfig.FormatterSettings["line_ending"]; !ok {
			c.Config.FormatterConfig.FormatterSettings["line_ending"] = c.Config.LineEnding
		}
	}

	factory, err := c.Registry.GetFactory(factoryType)
	if err != nil {
		return nil, err
	}
	return factory.NewFormatter(c.Config.FormatterConfig.FormatterSettings)
}

func (c *Command) collectPaths() ([]string, error) {
	collector, err := c.makePathCollector()
	if err != nil {
		return nil, err
	}

	return collector.CollectPaths()
}

func (c *Command) analyzePaths(paths []string) ([]string, error) {
	analyzer, err := c.makeAnalyzer()
	if err != nil {
		return nil, err
	}
	includePaths, _, err := analyzer.ExcludePathsByContent(paths)
	return includePaths, err
}

func (c *Command) makePathCollector() (yamlfmt.PathCollector, error) {
	switch c.Config.MatchType {
	case yamlfmt.MatchTypeDoublestar:
		return &yamlfmt.DoublestarCollector{
			Include: c.Config.Include,
			Exclude: c.Config.Exclude,
		}, nil
	case yamlfmt.MatchTypeGitignore:
		files := c.Config.Include
		if len(files) == 0 {
			files = []string{yamlfmt.DefaultPatternFile}
		}

		patternFile, err := yamlfmt.NewPatternFileCollector(files...)
		if err != nil {
			return nil, fmt.Errorf("NewPatternFile(%q): %w", files, err)
		}

		return patternFile, nil
	default:
		return &yamlfmt.FilepathCollector{
			Include:    c.Config.Include,
			Exclude:    c.Config.Exclude,
			Extensions: c.Config.Extensions,
		}, nil
	}
}

func (c *Command) makeAnalyzer() (yamlfmt.ContentAnalyzer, error) {
	return yamlfmt.NewBasicContentAnalyzer(c.Config.RegexExclude)
}

func readFromStdin() ([]byte, error) {
	stdin := bufio.NewReader(os.Stdin)
	data := []byte{}
	for {
		b, err := stdin.ReadByte()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}
		data = append(data, b)
	}
	return data, nil
}
