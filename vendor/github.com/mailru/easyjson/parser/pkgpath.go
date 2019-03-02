package parser

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

func getPkgPath(fname string, isDir bool) (string, error) {
	if !filepath.IsAbs(fname) {
		pwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		fname = filepath.Join(pwd, fname)
	}

	goModPath, _ := goModPath(fname, isDir)
	if strings.Contains(goModPath, "go.mod") {
		pkgPath, err := getPkgPathFromGoMod(fname, isDir, goModPath)
		if err != nil {
			return "", err
		}

		return pkgPath, nil
	}

	return getPkgPathFromGOPATH(fname, isDir)
}

var goModPathCache = struct {
	paths map[string]string
	sync.RWMutex
}{
	paths: make(map[string]string),
}

// empty if no go.mod, GO111MODULE=off or go without go modules support
func goModPath(fname string, isDir bool) (string, error) {
	root := fname
	if !isDir {
		root = filepath.Dir(fname)
	}

	goModPathCache.RLock()
	goModPath, ok := goModPathCache.paths[root]
	goModPathCache.RUnlock()
	if ok {
		return goModPath, nil
	}

	defer func() {
		goModPathCache.Lock()
		goModPathCache.paths[root] = goModPath
		goModPathCache.Unlock()
	}()

	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = root

	stdout, err := cmd.Output()
	if err != nil {
		return "", err
	}

	goModPath = string(bytes.TrimSpace(stdout))

	return goModPath, nil
}

func getPkgPathFromGoMod(fname string, isDir bool, goModPath string) (string, error) {
	modulePath := getModulePath(goModPath)
	if modulePath == "" {
		return "", fmt.Errorf("cannot determine module path from %s", goModPath)
	}

	rel := path.Join(modulePath, filePathToPackagePath(strings.TrimPrefix(fname, filepath.Dir(goModPath))))

	if !isDir {
		return path.Dir(rel), nil
	}

	return path.Clean(rel), nil
}

var (
	modulePrefix          = []byte("\nmodule ")
	pkgPathFromGoModCache = make(map[string]string)
)

func getModulePath(goModPath string) string {
	pkgPath, ok := pkgPathFromGoModCache[goModPath]
	if ok {
		return pkgPath
	}

	defer func() {
		pkgPathFromGoModCache[goModPath] = pkgPath
	}()

	data, err := ioutil.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	var i int
	if bytes.HasPrefix(data, modulePrefix[1:]) {
		i = 0
	} else {
		i = bytes.Index(data, modulePrefix)
		if i < 0 {
			return ""
		}
		i++
	}
	line := data[i:]

	// Cut line at \n, drop trailing \r if present.
	if j := bytes.IndexByte(line, '\n'); j >= 0 {
		line = line[:j]
	}
	if line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	line = line[len("module "):]

	// If quoted, unquote.
	pkgPath = strings.TrimSpace(string(line))
	if pkgPath != "" && pkgPath[0] == '"' {
		s, err := strconv.Unquote(pkgPath)
		if err != nil {
			return ""
		}
		pkgPath = s
	}
	return pkgPath
}

func getPkgPathFromGOPATH(fname string, isDir bool) (string, error) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		var err error
		gopath, err = getDefaultGoPath()
		if err != nil {
			return "", fmt.Errorf("cannot determine GOPATH: %s", err)
		}
	}

	for _, p := range strings.Split(gopath, string(filepath.ListSeparator)) {
		prefix := filepath.Join(p, "src") + string(filepath.Separator)
		if rel := strings.TrimPrefix(fname, prefix); rel != fname {
			if !isDir {
				return path.Dir(filePathToPackagePath(rel)), nil
			} else {
				return path.Clean(filePathToPackagePath(rel)), nil
			}
		}
	}

	return "", fmt.Errorf("file '%v' is not in GOPATH", fname)
}

func filePathToPackagePath(path string) string {
	return filepath.ToSlash(path)
}
