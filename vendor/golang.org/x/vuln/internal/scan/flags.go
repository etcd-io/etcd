// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/tools/go/buildutil"
	"golang.org/x/vuln/internal/govulncheck"
)

type config struct {
	govulncheck.Config
	patterns []string
	db       string
	dir      string
	tags     buildutil.TagsFlag
	test     bool
	show     ShowFlag
	format   FormatFlag
	version  bool
	env      []string
}

func parseFlags(cfg *config, stderr io.Writer, args []string) error {
	var version bool
	var json bool
	var scanFlag ScanFlag
	var modeFlag ModeFlag
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&json, "json", false, "output JSON (Go compatible legacy flag, see format flag)")
	flags.BoolVar(&cfg.test, "test", false, "analyze test files (only valid for source mode, default false)")
	flags.StringVar(&cfg.dir, "C", "", "change to `dir` before running govulncheck")
	flags.StringVar(&cfg.db, "db", "https://vuln.go.dev", "vulnerability database `url`")
	flags.Var(&modeFlag, "mode", "supports 'source', 'binary', and 'extract' (default 'source')")
	flags.Var(&cfg.tags, "tags", "comma-separated `list` of build tags")
	flags.Var(&cfg.show, "show", "enable display of additional information specified by the comma separated `list`\nThe supported values are 'traces','color', 'version', and 'verbose'")
	flags.Var(&cfg.format, "format", "specify format output\nThe supported values are 'text', 'json', 'sarif', and 'openvex' (default 'text')")
	flags.BoolVar(&version, "version", false, "print the version information")
	flags.Var(&scanFlag, "scan", "set the scanning level desired, one of 'module', 'package', or 'symbol' (default 'symbol')")

	// We don't want to print the whole usage message on each flags
	// error, so we set to a no-op and do the printing ourselves.
	flags.Usage = func() {}
	usage := func() {
		fmt.Fprint(flags.Output(), `Govulncheck reports known vulnerabilities in dependencies.

Usage:

	govulncheck [flags] [patterns]
	govulncheck -mode=binary [flags] [binary]

`)
		flags.PrintDefaults()
		fmt.Fprintf(flags.Output(), "\n%s\n", detailsMessage)
	}

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			usage() // print usage only on help
			return errHelp
		}
		return errUsage
	}
	cfg.patterns = flags.Args()
	if version {
		cfg.show = append(cfg.show, "version")
		cfg.version = true
	}
	cfg.ScanLevel = govulncheck.ScanLevel(scanFlag)
	cfg.ScanMode = govulncheck.ScanMode(modeFlag)
	if err := validateConfig(cfg, json); err != nil {
		fmt.Fprintln(flags.Output(), err)
		return errUsage
	}
	return nil
}

func validateConfig(cfg *config, json bool) error {
	// take care of default values
	if cfg.ScanMode == "" {
		cfg.ScanMode = govulncheck.ScanModeSource
	}
	if cfg.ScanLevel == "" {
		cfg.ScanLevel = govulncheck.ScanLevelSymbol
	}
	if json {
		if cfg.format != formatUnset {
			return fmt.Errorf("the -json flag cannot be used with -format flag")
		}
		cfg.format = formatJSON
	} else {
		if cfg.format == formatUnset {
			cfg.format = formatText
		}
	}

	// show flag is only supported with text output
	if cfg.format != formatText && len(cfg.show) > 0 {
		return fmt.Errorf("the -show flag is not supported for %s output", cfg.format)
	}

	switch cfg.ScanMode {
	case govulncheck.ScanModeSource:
		if len(cfg.patterns) == 1 && isFile(cfg.patterns[0]) {
			return fmt.Errorf("%q is a file.\n\n%v", cfg.patterns[0], errNoBinaryFlag)
		}
		if cfg.ScanLevel == govulncheck.ScanLevelModule && len(cfg.patterns) != 0 {
			return fmt.Errorf("patterns are not accepted for module only scanning")
		}
	case govulncheck.ScanModeBinary:
		if cfg.test {
			return fmt.Errorf("the -test flag is not supported in binary mode")
		}
		if len(cfg.tags) > 0 {
			return fmt.Errorf("the -tags flag is not supported in binary mode")
		}
		if len(cfg.patterns) != 1 {
			return fmt.Errorf("only 1 binary can be analyzed at a time")
		}
		if !isFile(cfg.patterns[0]) {
			return fmt.Errorf("%q is not a file", cfg.patterns[0])
		}
	case govulncheck.ScanModeExtract:
		if cfg.test {
			return fmt.Errorf("the -test flag is not supported in extract mode")
		}
		if len(cfg.tags) > 0 {
			return fmt.Errorf("the -tags flag is not supported in extract mode")
		}
		if len(cfg.patterns) != 1 {
			return fmt.Errorf("only 1 binary can be extracted at a time")
		}
		if cfg.format == formatJSON {
			return fmt.Errorf("the json format must be off in extract mode")
		}
		if !isFile(cfg.patterns[0]) {
			return fmt.Errorf("%q is not a file (source extraction is not supported)", cfg.patterns[0])
		}
	case govulncheck.ScanModeConvert:
		if len(cfg.patterns) != 0 {
			return fmt.Errorf("patterns are not accepted in convert mode")
		}
		if cfg.dir != "" {
			return fmt.Errorf("the -C flag is not supported in convert mode")
		}
		if cfg.test {
			return fmt.Errorf("the -test flag is not supported in convert mode")
		}
		if len(cfg.tags) > 0 {
			return fmt.Errorf("the -tags flag is not supported in convert mode")
		}
	case govulncheck.ScanModeQuery:
		if cfg.test {
			return fmt.Errorf("the -test flag is not supported in query mode")
		}
		if len(cfg.tags) > 0 {
			return fmt.Errorf("the -tags flag is not supported in query mode")
		}
		if cfg.format != formatJSON {
			return fmt.Errorf("the json format must be set in query mode")
		}
		for _, pattern := range cfg.patterns {
			// Parse the input here so that we can catch errors before
			// outputting the Config.
			if _, _, err := parseModuleQuery(pattern); err != nil {
				return err
			}
		}
	}
	return nil
}

func isFile(path string) bool {
	s, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !s.IsDir()
}

var errFlagParse = errors.New("see -help for details")

// ShowFlag is used for parsing and validation of
// govulncheck -show flag.
type ShowFlag []string

var supportedShows = map[string]bool{
	"traces":  true,
	"color":   true,
	"verbose": true,
	"version": true,
}

func (v *ShowFlag) Set(s string) error {
	if s == "" {
		return nil
	}
	for _, show := range strings.Split(s, ",") {
		sh := strings.TrimSpace(show)
		if _, ok := supportedShows[sh]; !ok {
			return errFlagParse
		}
		*v = append(*v, sh)
	}
	return nil
}

func (v *ShowFlag) Get() interface{} { return *v }
func (v *ShowFlag) String() string   { return "" }

// Update the text handler h with values of the flag.
func (v ShowFlag) Update(h *TextHandler) {
	for _, show := range v {
		switch show {
		case "traces":
			h.showTraces = true
		case "color":
			h.showColor = true
		case "version":
			h.showVersion = true
		case "verbose":
			h.showVerbose = true
		}
	}
}

// FormatFlag is used for parsing and validation of
// govulncheck -format flag.
type FormatFlag string

const (
	formatUnset   = ""
	formatJSON    = "json"
	formatText    = "text"
	formatSarif   = "sarif"
	formatOpenVEX = "openvex"
)

var supportedFormats = map[string]bool{
	formatJSON:    true,
	formatText:    true,
	formatSarif:   true,
	formatOpenVEX: true,
}

func (f *FormatFlag) Get() interface{} { return *f }
func (f *FormatFlag) Set(s string) error {
	if _, ok := supportedFormats[s]; !ok {
		return errFlagParse
	}
	*f = FormatFlag(s)
	return nil
}
func (f *FormatFlag) String() string { return "" }

// ModeFlag is used for parsing and validation of
// govulncheck -mode flag.
type ModeFlag string

var supportedModes = map[string]bool{
	govulncheck.ScanModeSource:  true,
	govulncheck.ScanModeBinary:  true,
	govulncheck.ScanModeConvert: true,
	govulncheck.ScanModeQuery:   true,
	govulncheck.ScanModeExtract: true,
}

func (f *ModeFlag) Get() interface{} { return *f }
func (f *ModeFlag) Set(s string) error {
	if _, ok := supportedModes[s]; !ok {
		return errFlagParse
	}
	*f = ModeFlag(s)
	return nil
}
func (f *ModeFlag) String() string { return "" }

// ScanFlag is used for parsing and validation of
// govulncheck -scan flag.
type ScanFlag string

var supportedLevels = map[string]bool{
	govulncheck.ScanLevelModule:  true,
	govulncheck.ScanLevelPackage: true,
	govulncheck.ScanLevelSymbol:  true,
}

func (f *ScanFlag) Get() interface{} { return *f }
func (f *ScanFlag) Set(s string) error {
	if _, ok := supportedLevels[s]; !ok {
		return errFlagParse
	}
	*f = ScanFlag(s)
	return nil
}
func (f *ScanFlag) String() string { return "" }
