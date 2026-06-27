package fsutils

import (
	"bytes"
	"fmt"
	"sync"
)

type fileLinesCache [][]byte

type LineCache struct {
	files     sync.Map
	fileCache *FileCache
}

func NewLineCache(fc *FileCache) *LineCache {
	return &LineCache{
		fileCache: fc,
	}
}

// GetLine returns the index1-th (1-based index) line from the file on filePath
func (lc *LineCache) GetLine(filePath string, index1 int) (string, error) {
	if index1 == 0 { // some linters, e.g. gosec can do it: it really means first line
		index1 = 1
	}

	const index1To0Offset = -1
	rawLine, err := lc.getRawLine(filePath, index1+index1To0Offset)
	if err != nil {
		return "", err
	}

	return string(bytes.Trim(rawLine, "\r")), nil
}

func (lc *LineCache) getRawLine(filePath string, index0 int) ([]byte, error) {
	fc, err := lc.getFileCache(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file %s lines cache: %w", filePath, err)
	}

	if index0 < 0 {
		return nil, fmt.Errorf("invalid file line index0 < 0: %d", index0)
	}

	if index0 >= len(fc) {
		return nil, fmt.Errorf("invalid file line index0 (%d) >= len(fc) (%d)", index0, len(fc))
	}

	return fc[index0], nil
}

func (lc *LineCache) getFileCache(filePath string) (fileLinesCache, error) {
	loadedFc, ok := lc.files.Load(filePath)
	if ok {
		return loadedFc.(fileLinesCache), nil
	}

	fileBytes, err := lc.fileCache.GetFileBytes(filePath)
	if err != nil {
		return nil, fmt.Errorf("can't get file %s bytes from cache: %w", filePath, err)
	}

	fc := bytes.Split(fileBytes, []byte("\n"))
	lc.files.Store(filePath, fileLinesCache(fc))
	return fc, nil
}
