//go:build windows
// +build windows

package colorprofile

import (
	"strconv"

	"golang.org/x/sys/windows"
)

func windowsColorProfile(env map[string]string) (Profile, bool) {
	if env["ConEmuANSI"] == "ON" {
		return TrueColor, true
	}

	major, _, build := windows.RtlGetNtVersionNumbers()
	if build < 10586 || major < 10 {
		// No ANSI support before WindowsNT 10 build 10586
		if len(env["ANSICON"]) > 0 {
			ansiconVer := env["ANSICON_VER"]
			cv, err := strconv.Atoi(ansiconVer)
			if err != nil || cv < 181 {
				// No 8 bit color support before ANSICON 1.81
				return ANSI, true
			}

			return ANSI256, true
		}

		return NoTTY, true
	}

	if build < 14931 {
		// No true color support before build 14931
		return ANSI256, true
	}

	return TrueColor, true
}
