package stdinfo

type Package struct {
	// Name is a package name.
	// For "encoding/json" the package name is "json".
	Name string

	// Path is a package path, like "encoding/json".
	Path string

	// Freq is a package import frequency approximation.
	// A value of -1 means "unknown".
	Freq int
}

// PathByName maps a std package name to its package path.
//
// For packages with multiple choices, like "template",
// only the more common one is accessible ("text/template" in this case).
//
// This map doesn't contain extremely rare packages either.
// Use PackageList variable if you want to construct a different mapping.
//
// It's exported as map to make it easier to re-use it in libraries
// without copying.
var PathByName = generatedPathByName

// PackagesList is a list of std packages information.
// It's sorted by a package name.
var PackagesList = generatedPackagesList
