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

package main

import (
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/engine"
)

var (
	flagLint *bool = flag.Bool("lint", false, `Check if there are any differences between
source yaml and formatted yaml.`)
	flagDry *bool = flag.Bool("dry", false, `Perform a dry run; show the output of a formatting
operation without performing it.`)
	flagIn                *bool   = flag.Bool("in", false, "Format yaml read from stdin and output to stdout")
	flagVersion           *bool   = flag.Bool("version", false, "Print yamlfmt version")
	flagConf              *string = flag.String("conf", "", "Read yamlfmt config from this path")
	flagGlobalConf        *bool   = flag.Bool("global_conf", false, fmt.Sprintf("Use global yamlfmt config from %s", globalConfFlagVar()))
	flagDisableGlobalConf *bool   = flag.Bool("no_global_conf", false, fmt.Sprintf("Disabled usage of global yamlfmt config from %s", globalConfFlagVar()))
	flagPrintConf         *bool   = flag.Bool("print_conf", false, "Print config")
	flagDoublestar        *bool   = flag.Bool("dstar", false, "Use doublestar globs for include and exclude")
	flagQuiet             *bool   = flag.Bool("quiet", false, "Print minimal output to stdout")
	flagQuietShort        *bool   = flag.Bool("q", false, "Print minimal output to stdout")
	flagVerbose           *bool   = flag.Bool("verbose", false, "Print additional output to stdout.")
	flagVerboseShort      *bool   = flag.Bool("v", false, "Print additional output to stdout.")
	flagContinueOnError   *bool   = flag.Bool("continue_on_error", false, "Continue to format files that didn't fail instead of exiting with code 1.")
	flagGitignoreExcludes *bool   = flag.Bool("gitignore_excludes", false, "Use a gitignore file for excludes")
	flagGitignorePath     *string = flag.String("gitignore_path", ".gitignore", "Path to gitignore file to use")
	flagOutputFormat      *string = flag.String("output_format", "default", "The engine output format")
	flagMatchType         *string = flag.String("match_type", "", "The file discovery method to use. Valid values: standard, doublestar, gitignore")
	flagKyaml             *bool   = flag.Bool("kyaml", false, "Flag to switch to kyaml formatting. If used, all formatter configuration from detected from configuration file is overridden.")
	flagExclude                   = arrayFlag{}
	flagFormatter                 = arrayFlag{}
	flagExtensions                = arrayFlag{}
	flagDebug                     = arrayFlag{}
)

func bindArrayFlags() {
	flag.Var(&flagExclude, "exclude", "Paths to exclude in the chosen format (standard or doublestar)")
	flag.Var(&flagFormatter, "formatter", "Config value overrides to pass to the formatter")
	flag.Var(&flagExtensions, "extensions", "File extensions to use for standard path collection")
	flag.Var(&flagDebug, "debug", "Debug codes to activate for debug logging")
}

type arrayFlag []string

// Implements flag.Value
func (a *arrayFlag) String() string {
	return strings.Join(*a, " ")
}

func (a *arrayFlag) Set(value string) error {
	values := []string{value}
	if strings.Contains(value, ",") {
		values = strings.Split(value, ",")
	}
	*a = append(*a, values...)
	return nil
}

func configureHelp() {
	flag.Usage = func() {
		fmt.Println(`yamlfmt is a simple command line tool for formatting yaml files.

	Arguments:

	Glob paths to yaml files
			Send any number of paths to yaml files specified in doublestar glob format (see: https://github.com/bmatcuk/doublestar).
			Any flags must be specified before the paths.

	- or /dev/stdin
			Passing in a single - or /dev/stdin will read the yaml from stdin and output the formatted result to stdout

	Flags:`)
		flag.PrintDefaults()
	}
}

func getOperationFromFlag() yamlfmt.Operation {
	if *flagIn || isStdinArg() {
		return yamlfmt.OperationStdin
	}
	if *flagLint {
		return yamlfmt.OperationLint
	}
	if *flagDry {
		return yamlfmt.OperationDry
	}
	if *flagPrintConf {
		return yamlfmt.OperationPrintConfig
	}
	return yamlfmt.OperationFormat
}

func getOutputFormatFromFlag() engine.EngineOutputFormat {
	return engine.EngineOutputFormat(*flagOutputFormat)
}

func isStdinArg() bool {
	if len(flag.Args()) != 1 {
		return false
	}
	arg := flag.Args()[0]
	return arg == "-" || arg == "/dev/stdin"
}

func globalConfFlagVar() string {
	if runtime.GOOS == "windows" {
		return "LOCALAPPDATA"
	}
	return "XDG_CONFIG_HOME"
}
