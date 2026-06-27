// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/vuln/internal"
	"golang.org/x/vuln/internal/govulncheck"
)

// validateFindings checks that the supplied findings all obey the protocol
// rules.
func validateFindings(findings ...*govulncheck.Finding) error {
	for _, f := range findings {
		if f.OSV == "" {
			return fmt.Errorf("invalid finding: all findings must have an associated OSV")
		}
		if len(f.Trace) < 1 {
			return fmt.Errorf("invalid finding: all callstacks must have at least one frame")
		}
		for _, frame := range f.Trace {
			if frame.Version != "" && frame.Module == "" {
				return fmt.Errorf("invalid finding: if Frame.Version (%s) is set, Frame.Module must also be", frame.Version)
			}
			if frame.Package != "" && frame.Module == "" {
				return fmt.Errorf("invalid finding: if Frame.Package (%s) is set, Frame.Module must also be", frame.Package)
			}
			if frame.Function != "" && frame.Package == "" {
				return fmt.Errorf("invalid finding: if Frame.Function (%s) is set, Frame.Package must also be", frame.Function)
			}
		}
	}
	return nil
}

func moduleVersionString(modulePath, version string) string {
	if version == "" {
		return ""
	}
	if modulePath == internal.GoStdModulePath || modulePath == internal.GoCmdModulePath {
		version = semverToGoTag(version)
	}
	return version
}

func gomodExists(dir string) bool {
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	output := strings.TrimSpace(string(out))
	// If module-aware mode is enabled, but there is no go.mod, GOMOD will be os.DevNull
	// If module-aware mode is disabled, GOMOD will be the empty string.
	return err == nil && !(output == os.DevNull || output == "")
}
