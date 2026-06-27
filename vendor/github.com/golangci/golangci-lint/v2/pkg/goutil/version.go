package goutil

import (
	"fmt"
	"go/version"
	"regexp"
	"runtime"
	"strings"

	hcversion "github.com/hashicorp/go-version"
)

func CheckGoVersion(goVersion string) error {
	rv, err := CleanRuntimeVersion()
	if err != nil {
		return fmt.Errorf("clean runtime version: %w", err)
	}

	langVersion := version.Lang(rv)

	runtimeVersion, err := hcversion.NewVersion(strings.TrimPrefix(langVersion, "go"))
	if err != nil {
		return err
	}

	targetedVersion, err := hcversion.NewVersion(TrimGoVersion(goVersion))
	if err != nil {
		return err
	}

	if runtimeVersion.LessThan(targetedVersion) {
		return fmt.Errorf("the Go language version (%s) used to build golangci-lint is lower than the targeted Go version (%s)",
			langVersion, goVersion)
	}

	return nil
}

// TrimGoVersion Trims the Go version to keep only M.m.
// Since Go 1.21 the version inside the go.mod can be a patched version (ex: 1.21.0).
// The version can also include information which we want to remove (ex: 1.21alpha1)
// https://go.dev/doc/toolchain#versions
// This a problem with staticcheck and gocritic.
func TrimGoVersion(v string) string {
	if v == "" {
		return ""
	}

	exp := regexp.MustCompile(`(\d\.\d+)(?:\.\d+|[a-z]+\d)`)

	if exp.MatchString(v) {
		return exp.FindStringSubmatch(v)[1]
	}

	return v
}

func CleanRuntimeVersion() (string, error) {
	return cleanRuntimeVersion(runtime.Version())
}

func cleanRuntimeVersion(rv string) (string, error) {
	for part := range strings.FieldsSeq(rv) {
		// Allow to handle:
		// - GOEXPERIMENT -> "go1.23.0 X:boringcrypto"
		// - devel -> "devel go1.24-e705a2d Wed Aug 7 01:16:42 2024 +0000 linux/amd64"
		if strings.HasPrefix(part, "go1.") {
			return part, nil
		}
	}

	return "", fmt.Errorf("invalid Go runtime version: %s", rv)
}
