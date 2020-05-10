package gopkgs

import (
	"bufio"
	"bytes"
	"errors"
	"go/build"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/karrick/godirwalk"
	pkgerrors "github.com/pkg/errors"
)

// Pkg hold the information of the package.
type Pkg struct {
	Dir        string // directory containing package sources
	ImportPath string // import path of package in dir
	Name       string // package name
}

// Options for retrieve packages.
type Options struct {
	WorkDir  string // Will return importable package under WorkDir. Any vendor dependencies outside the WorkDir will be ignored.
	NoVendor bool   // Will not retrieve vendor dependencies, except inside WorkDir (if specified)
}

type goFile struct {
	path string
	dir  string
}

func mustClose(c io.Closer) {
	if err := c.Close(); err != nil {
		panic(err)
	}
}

func readPackageName(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}

	s := bufio.NewScanner(f)
	var inComment bool
	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if line == "" {
			continue
		}

		if !inComment {
			if strings.HasPrefix(line, "/*") {
				inComment = true
				continue
			}

			if strings.HasPrefix(line, "//") {
				// skip inline comment
				continue
			}

			if strings.HasPrefix(line, "package") {
				ls := strings.Split(line, " ")
				if len(ls) < 2 {
					mustClose(f)
					return "", errors.New("expect pattern 'package <name>':" + line)
				}

				mustClose(f)
				return ls[1], nil
			}

			// package should be found first
			mustClose(f)
			return "", errors.New("invalid go file, expect package declaration")
		}

		// inComment = true
		if strings.HasSuffix(line, "*/") {
			inComment = false
		}
	}

	mustClose(f)
	return "", errors.New("cannot find package information")
}

func listFiles(srcDir, workDir string, noVendor bool) (<-chan goFile, <-chan error) {
	filec := make(chan goFile, 10000)
	errc := make(chan error, 1)

	go func() {
		defer func() {
			close(filec)
			close(errc)
		}()

		if workDir != "" && !filepath.IsAbs(workDir) {
			wd, err := filepath.Abs(workDir)
			if err != nil {
				errc <- err
				return
			}

			workDir = wd
		}

		err := godirwalk.Walk(srcDir, &godirwalk.Options{
			FollowSymbolicLinks: true,
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				name := de.Name()
				pathDir := filepath.Dir(osPathname)

				// Symlink not supported by go
				if de.IsSymlink() {
					return filepath.SkipDir
				}

				// Ignore files begin with "_", "." "_test.go" and directory named "testdata"
				// see: https://golang.org/cmd/go/#hdr-Description_of_package_lists

				if de.IsDir() {
					if name[0] == '.' || name[0] == '_' || name == "testdata" || name == "node_modules" {
						return filepath.SkipDir
					}

					if name == "vendor" {
						if workDir != "" {
							if !visibleVendor(workDir, pathDir) {
								return filepath.SkipDir
							}

							return nil
						}

						if noVendor {
							return filepath.SkipDir
						}
					}

					return nil
				}

				if name[0] == '.' || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					return nil
				}

				if pathDir == srcDir {
					// Cannot put files on $GOPATH/src or $GOROOT/src.
					return nil
				}

				filec <- goFile{
					path: osPathname,
					dir:  pathDir,
				}
				return nil
			},
			ErrorCallback: func(s string, err error) godirwalk.ErrorAction {
				err = pkgerrors.Cause(err)
				if v, ok := err.(*os.PathError); ok && os.IsNotExist(v.Err) {
					return godirwalk.SkipNode
				}

				return godirwalk.Halt
			},
		})

		if err != nil {
			errc <- err
			return
		}
	}()
	return filec, errc
}

func listModFiles(modDir string) (<-chan goFile, <-chan error) {
	filec := make(chan goFile, 10000)
	errc := make(chan error, 1)

	go func() {
		defer func() {
			close(filec)
			close(errc)
		}()

		err := godirwalk.Walk(modDir, &godirwalk.Options{
			FollowSymbolicLinks: true,
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				name := de.Name()
				pathDir := filepath.Dir(osPathname)

				// Symlink not supported by go
				if de.IsSymlink() {
					return filepath.SkipDir
				}

				// Ignore files begin with "_", "." "_test.go" and directory named "testdata"
				// see: https://golang.org/cmd/go/#hdr-Description_of_package_lists

				if de.IsDir() {
					if name[0] == '.' || name[0] == '_' || name == "testdata" || name == "node_modules" {
						return filepath.SkipDir
					}

					return nil
				}

				if name[0] == '.' || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					return nil
				}

				filec <- goFile{
					path: osPathname,
					dir:  pathDir,
				}
				return nil
			},
			ErrorCallback: func(s string, err error) godirwalk.ErrorAction {
				err = pkgerrors.Cause(err)
				if v, ok := err.(*os.PathError); ok && os.IsNotExist(v.Err) {
					return godirwalk.SkipNode
				}

				return godirwalk.Halt
			},
		})

		if err != nil {
			errc <- err
			return
		}
	}()
	return filec, errc
}

func collectPkgs(srcDir, workDir string, noVendor bool, out map[string]Pkg) error {
	filec, errc := listFiles(srcDir, workDir, noVendor)
	for f := range filec {
		pkgDir := f.dir
		if _, found := out[pkgDir]; found {
			// already have this package, skip
			continue
		}

		pkgName, err := readPackageName(f.path)
		if err != nil {
			// skip unparseable file
			continue
		}

		if pkgName == "main" {
			// skip main package
			continue
		}

		out[pkgDir] = Pkg{
			Name:       pkgName,
			ImportPath: filepath.ToSlash(pkgDir[len(srcDir)+len("/"):]),
			Dir:        pkgDir,
		}
	}

	if err := <-errc; err != nil {
		return err
	}

	return nil
}

func collectModPkgs(m mod, out map[string]Pkg) error {
	filec, errc := listModFiles(m.dir)
	for f := range filec {
		pkgDir := f.dir
		if _, found := out[pkgDir]; found {
			// already have this package, skip
			continue
		}

		pkgName, err := readPackageName(f.path)
		if err != nil {
			// skip unparseable file
			continue
		}

		if pkgName == "main" {
			// skip main package
			continue
		}

		// debug := true
		importPath := m.path
		if pkgDir != m.dir {
			importPath += filepath.ToSlash(pkgDir[len(m.dir):])
		}

		out[pkgDir] = Pkg{
			Name:       pkgName,
			ImportPath: importPath,
			Dir:        pkgDir,
		}
	}

	if err := <-errc; err != nil {
		return err
	}

	return nil
}

// List packages on workDir.
// workDir is required for module mode. If the workDir is not under module, then it will fallback to GOPATH mode.
func List(opts Options) (map[string]Pkg, error) {
	pkgs := make(map[string]Pkg)

	if opts.WorkDir == "" {
		// force on GOPATH mode
		for _, srcDir := range build.Default.SrcDirs() {
			err := collectPkgs(srcDir, opts.WorkDir, opts.NoVendor, pkgs)
			if err != nil {
				return nil, err
			}
		}
		return pkgs, nil
	}

	mods, err := listMods(opts.WorkDir)
	if err != nil {
		// GOPATH mode
		for _, srcDir := range build.Default.SrcDirs() {
			err = collectPkgs(srcDir, opts.WorkDir, opts.NoVendor, pkgs)
			if err != nil {
				return nil, err
			}
		}
		return pkgs, nil
	}

	// Module mode
	if err = collectPkgs(filepath.Join(build.Default.GOROOT, "src"), opts.WorkDir, false, pkgs); err != nil {
		return nil, err
	}

	for _, m := range mods {
		err = collectModPkgs(m, pkgs)
		if err != nil {
			return nil, err
		}
	}

	return pkgs, nil
}

type mod struct {
	path string
	dir  string
}

func listMods(workDir string) ([]mod, error) {
	cmdArgs := []string{"list", "-m", "-f={{.Path}};{{.Dir}}", "all"}
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var mods []mod
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		line := s.Text()
		ls := strings.Split(line, ";")
		mods = append(mods, mod{path: ls[0], dir: ls[1]})
	}
	return mods, nil
}
