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
	"log"
	"os"
	"runtime/debug"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/command"
	"github.com/google/yamlfmt/formatters/basic"
	"github.com/google/yamlfmt/formatters/kyaml"
	"github.com/google/yamlfmt/internal/logger"
)

var (
	version = ""
	commit  = ""
)

func main() {
	if err := run(); err != nil {
		l := log.New(os.Stderr, "", 0)
		l.Fatal(err)
	}
}

func run() error {
	bindArrayFlags()
	configureHelp()
	flag.Parse()

	if *flagVersion {
		currentVersion, currentCommit := getVersion()
		versionMessage := fmt.Sprintf("yamlfmt %s", currentVersion)
		if currentCommit != "" {
			versionMessage += fmt.Sprintf(" (%s)", currentCommit)
		}
		fmt.Println(versionMessage)
		return nil
	}

	for _, code := range flagDebug {
		logger.ActivateDebugCode(code)
	}

	c := &command.Command{
		Operation: getOperationFromFlag(),
		Registry:  getFullRegistry(),
		Quiet:     *flagQuiet || *flagQuietShort,
		Verbose:   *flagVerbose || *flagVerboseShort,
	}

	configData := map[string]any{}
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	if configPath != "" {
		configData, err = readConfig(configPath)
		if err != nil {
			return err
		}
	} else if len(os.Args) == 1 {
		// If the user doesn't have a yamlfmt config and didn't provide
		// any arguments, the command is destined to no-op. Provide the
		// default help message to indicate proper usage.
		flag.Usage()
		return nil
	}

	commandConfig, err := makeCommandConfigFromData(configData)
	if err != nil {
		return err
	}
	c.Config = commandConfig

	return c.Run()
}

func getFullRegistry() *yamlfmt.Registry {
	registry := yamlfmt.NewFormatterRegistry(&basic.BasicFormatterFactory{})
	registry.Add(&kyaml.KYAMLFormatterFactory{})
	return registry
}

func getVersion() (string, string) {
	// If version was set via ldflags, return that.
	if version != "" {
		return version, commit
	}

	buildInfo, ok := debug.ReadBuildInfo()
	// If buildinfo can't be read, return default values.
	if !ok {
		return "dev", ""
	}

	// Otherwise read the version from buildInfo (this is to cover
	// the go install usecase).
	return buildInfo.Main.Version, ""
}
