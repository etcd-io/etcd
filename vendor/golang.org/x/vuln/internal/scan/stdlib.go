// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scan

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// Support functions for standard library packages.
// These are copied from the internal/stdlib package in the pkgsite repo.

// semverToGoTag returns the Go standard library repository tag corresponding
// to semver, a version string without the initial "v".
// Go tags differ from standard semantic versions in a few ways,
// such as beginning with "go" instead of "v".
func semverToGoTag(v string) string {
	if strings.HasPrefix(v, "v0.0.0") {
		return "master"
	}
	// Special case: v1.0.0 => go1.
	if v == "v1.0.0" {
		return "go1"
	}
	if !semver.IsValid(v) {
		return fmt.Sprintf("<!%s:invalid semver>", v)
	}
	goVersion := semver.Canonical(v)
	prerelease := semver.Prerelease(goVersion)
	versionWithoutPrerelease := strings.TrimSuffix(goVersion, prerelease)
	patch := strings.TrimPrefix(versionWithoutPrerelease, semver.MajorMinor(goVersion)+".")
	if patch == "0" {
		versionWithoutPrerelease = strings.TrimSuffix(versionWithoutPrerelease, ".0")
	}
	goVersion = fmt.Sprintf("go%s", strings.TrimPrefix(versionWithoutPrerelease, "v"))
	if prerelease != "" {
		// Go prereleases look like  "beta1" instead of "beta.1".
		// "beta1" is bad for sorting (since beta10 comes before beta9), so
		// require the dot form.
		i := finalDigitsIndex(prerelease)
		if i >= 1 {
			if prerelease[i-1] != '.' {
				return fmt.Sprintf("<!%s:final digits in a prerelease must follow a period>", v)
			}
			// Remove the dot.
			prerelease = prerelease[:i-1] + prerelease[i:]
		}
		goVersion += strings.TrimPrefix(prerelease, "-")
	}
	return goVersion
}

// finalDigitsIndex returns the index of the first digit in the sequence of digits ending s.
// If s doesn't end in digits, it returns -1.
func finalDigitsIndex(s string) int {
	// Assume ASCII (since the semver package does anyway).
	var i int
	for i = len(s) - 1; i >= 0; i-- {
		if s[i] < '0' || s[i] > '9' {
			break
		}
	}
	if i == len(s)-1 {
		return -1
	}
	return i + 1
}
