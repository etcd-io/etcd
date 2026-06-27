// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gosec

import (
	"go/ast"
	"go/types"
	"regexp"
	"strings"
)

var versioningPackagePattern = regexp.MustCompile(`v[0-9]+$`)

// ImportTracker is used to normalize the packages that have been imported
// by a source file. It is able to differentiate between plain imports, aliased
// imports and init only imports.
type ImportTracker struct {
	// Imported is a map of Imported with their associated names/aliases.
	Imported map[string][]string
}

// NewImportTracker creates an empty Import tracker instance
func NewImportTracker() *ImportTracker {
	return &ImportTracker{
		Imported: make(map[string][]string),
	}
}

// TrackFile track all the imports used by the supplied file
func (t *ImportTracker) TrackFile(file *ast.File) {
	for _, imp := range file.Imports {
		t.TrackImport(imp)
	}
}

// TrackPackages tracks all the imports used by the supplied packages
func (t *ImportTracker) TrackPackages(pkgs ...*types.Package) {
	for _, pkg := range pkgs {
		t.Imported[pkg.Path()] = []string{pkg.Name()}
	}
}

// TrackImport tracks imports.
func (t *ImportTracker) TrackImport(imported *ast.ImportSpec) {
	importPath := strings.Trim(imported.Path.Value, `"`)
	if imported.Name != nil {
		if imported.Name.Name != "_" {
			// Aliased import
			t.Imported[importPath] = append(t.Imported[importPath], imported.Name.String())
		}
	} else {
		t.Imported[importPath] = append(t.Imported[importPath], importName(importPath))
	}
}

func importName(importPath string) string {
	parts := strings.Split(importPath, "/")
	name := importPath
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}
	// If the last segment of the path is version information, consider the second to last segment as the package name.
	// (e.g., `math/rand/v2` would be `rand`)
	if len(parts) > 1 && versioningPackagePattern.MatchString(name) {
		name = parts[len(parts)-2]
	}
	return name
}
