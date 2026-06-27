package fsutils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

func IsDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

var (
	cachedWd      string
	cachedWdError error
	getWdOnce     sync.Once
	useCache      = true
)

func UseWdCache(use bool) {
	useCache = use
}

func Getwd() (string, error) {
	if !useCache { // for tests
		return os.Getwd()
	}

	getWdOnce.Do(func() {
		cachedWd, cachedWdError = os.Getwd()
		if cachedWdError != nil {
			return
		}

		evaluatedWd, err := EvalSymlinks(cachedWd)
		if err != nil {
			cachedWd, cachedWdError = "", fmt.Errorf("can't eval symlinks on wd %s: %w", cachedWd, err)
			return
		}

		cachedWd = evaluatedWd
	})

	return cachedWd, cachedWdError
}

var evalSymlinkCache sync.Map

type evalSymlinkRes struct {
	path string
	err  error
}

func EvalSymlinks(path string) (string, error) {
	r, ok := evalSymlinkCache.Load(path)
	if ok {
		er := r.(evalSymlinkRes)
		return er.path, er.err
	}

	var er evalSymlinkRes
	er.path, er.err = evalSymlinks(path)
	evalSymlinkCache.Store(path, er)

	return er.path, er.err
}

func ShortestRelPath(path, wd string) (string, error) {
	if wd == "" { // get it if user don't have cached working dir
		var err error
		wd, err = Getwd()
		if err != nil {
			return "", fmt.Errorf("can't get working directory: %w", err)
		}
	}

	evaluatedPath, err := EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("can't eval symlinks for path %s: %w", path, err)
	}
	path = evaluatedPath

	// make path absolute and then relative to be able to fix this case:
	// we are in `/test` dir, we want to normalize `../test`, and have file `file.go` in this dir;
	// it must have normalized path `file.go`, not `../test/file.go`,
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(wd, path)
	}

	relPath, err := filepath.Rel(wd, absPath)
	if err != nil {
		return "", fmt.Errorf("can't get relative path for path %s and root %s: %w",
			absPath, wd, err)
	}

	return relPath, nil
}
