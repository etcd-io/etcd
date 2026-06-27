package loader

import (
	"fmt"
	"runtime"
	"sort"
	"strings"

	"honnef.co/go/tools/go/buildid"
	"honnef.co/go/tools/lintcmd/cache"
)

// computeHash computes a package's hash. The hash is based on all Go
// files that make up the package, as well as the hashes of imported
// packages.
func computeHash(c *cache.Cache, pkg *PackageSpec) (cache.ActionID, error) {
	key := c.NewHash("package " + pkg.PkgPath)
	fmt.Fprintf(key, "goos %s goarch %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(key, "import %q\n", pkg.PkgPath)

	// Compute the hashes of all files making up the package. As an
	// optimization, we use the build ID that Go already computed for us,
	// because it is virtually identical to hashing all CompiledGoFiles. It is
	// also sensitive to the Go version declared in go.mod.
	success := false
	if pkg.ExportFile != "" {
		id, err := getBuildid(pkg.ExportFile)
		if err == nil {
			if idx := strings.IndexRune(id, '/'); idx > -1 {
				fmt.Fprintf(key, "files %s\n", id[:idx])
				success = true
			}
		}
	}
	if !success {
		for _, f := range pkg.CompiledGoFiles {
			h, err := cache.FileHash(f)
			if err != nil {
				return cache.ActionID{}, err
			}
			fmt.Fprintf(key, "file %s %x\n", f, h)
		}
		if pkg.Module != nil && pkg.Module.GoMod != "" {
			// The go.mod file specifies the language version, which affects how
			// packages are analyzed.
			h, err := cache.FileHash(pkg.Module.GoMod)
			if err != nil {
				// TODO(dh): this doesn't work for tests because the go.mod file doesn't
				// exist on disk and is instead provided via an overlay. However, we're
				// unlikely to get here in the first place, as reading the build ID from
				// the export file is likely to succeed.
				return cache.ActionID{}, fmt.Errorf("couldn't hash go.mod: %w", err)
			} else {
				fmt.Fprintf(key, "file %s %x\n", pkg.Module.GoMod, h)
			}
		}
	}

	imps := make([]*PackageSpec, 0, len(pkg.Imports))
	for _, v := range pkg.Imports {
		imps = append(imps, v)
	}
	sort.Slice(imps, func(i, j int) bool {
		return imps[i].PkgPath < imps[j].PkgPath
	})

	for _, dep := range imps {
		if dep.ExportFile == "" {
			fmt.Fprintf(key, "import %s \n", dep.PkgPath)
		} else {
			id, err := getBuildid(dep.ExportFile)
			if err == nil {
				fmt.Fprintf(key, "import %s %s\n", dep.PkgPath, id)
			} else {
				fh, err := cache.FileHash(dep.ExportFile)
				if err != nil {
					return cache.ActionID{}, err
				}
				fmt.Fprintf(key, "import %s %x\n", dep.PkgPath, fh)
			}
		}
	}
	return key.Sum(), nil
}

var buildidCache = map[string]string{}

func getBuildid(f string) (string, error) {
	if h, ok := buildidCache[f]; ok {
		return h, nil
	}
	h, err := buildid.ReadFile(f)
	if err != nil {
		return "", err
	}
	buildidCache[f] = h
	return h, nil
}
