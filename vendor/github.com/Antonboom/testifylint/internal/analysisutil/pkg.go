package analysisutil

import (
	"go/types"
	"strings"
)

// IsPkg checks that package has corresponding objName and path.
// Supports vendored packages.
func IsPkg(pkg *types.Package, name, path string) bool {
	return pkg.Name() == name && trimVendor(pkg.Path()) == path
}

func trimVendor(path string) string {
	if strings.HasPrefix(path, "vendor/") {
		return path[len("vendor/"):]
	}
	return path
}
