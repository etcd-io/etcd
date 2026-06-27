package io

import (
	"io/fs"
	"os"
	"path/filepath"
)

type fileCheckFunction func(path string, file os.FileInfo) bool

func FindFilesForPath(path string, fileCheckFun fileCheckFunction) ([]string, error) {
	switch entry, err := os.Stat(path); {
	case err != nil:
		return nil, err
	case entry.IsDir():
		return findFilesForDirectory(path, fileCheckFun)
	case fileCheckFun(path, entry):
		return []string{filepath.Clean(path)}, nil
	default:
		return []string{}, nil
	}
}

func findFilesForDirectory(dirPath string, fileCheckFun fileCheckFunction) ([]string, error) {
	var filePaths []string
	err := filepath.WalkDir(dirPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		file, err := entry.Info()
		if err != nil {
			return err
		}
		if !entry.IsDir() && fileCheckFun(path, file) {
			filePaths = append(filePaths, filepath.Clean(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return filePaths, nil
}

func isGoFile(_ string, file os.FileInfo) bool {
	return !file.IsDir() && filepath.Ext(file.Name()) == ".go"
}

func isOutsideVendorDir(path string, _ os.FileInfo) bool {
	for {
		base := filepath.Base(path)
		if base == "vendor" {
			return false
		}

		prevPath := path
		path = filepath.Dir(path)

		if prevPath == path {
			break
		}
	}

	return true
}

func checkChains(funcs ...fileCheckFunction) fileCheckFunction {
	return func(path string, file os.FileInfo) bool {
		for _, checkFunc := range funcs {
			if !checkFunc(path, file) {
				return false
			}
		}

		return true
	}
}
