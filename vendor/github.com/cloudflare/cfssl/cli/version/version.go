// Package version implements the version command.
package version

import (
	"fmt"
	"runtime"

	"github.com/cloudflare/cfssl/cli"
)

var (
	version = "dev"
)

// Usage text for 'cfssl version'
var versionUsageText = `cfssl version -- print out the version of CF SSL

Usage of version:
	cfssl version
`

// FormatVersion returns the formatted version string.
func FormatVersion() string {
	return fmt.Sprintf("Version: %s\nRuntime: %s\n", version, runtime.Version())
}

// The main functionality of 'cfssl version' is to print out the version info.
func versionMain(args []string, c cli.Config) (err error) {
	fmt.Printf("%s", FormatVersion())
	return nil
}

// Command assembles the definition of Command 'version'
var Command = &cli.Command{UsageText: versionUsageText, Flags: nil, Main: versionMain}
