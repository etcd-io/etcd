package gopkgs

import "strings"

func visibleVendor(workDir, vendorDir string) bool {
	return strings.Index(workDir, vendorDir) == 0
}
