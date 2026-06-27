//go:build !windows
// +build !windows

package colorprofile

func windowsColorProfile(map[string]string) (Profile, bool) {
	return 0, false
}
