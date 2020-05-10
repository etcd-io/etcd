// Copyright 2015 Auburn University. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file defines a FileSystem interface and two implementations.  A
// FileSystem is supplied to the go/loader to read files, and it is also used
// by the refactoring driver to commit refactorings' changes to disk.

// Package filesystem provides a file system abstraction and types describing
// potential changes to a file system.
package filesystem

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godoctor/godoctor/text"
)

// The filename assigned to Go source code supplied on standard input.  It is
// assumed that a file with this name does not actually exist.
//
// This peculiar choice of names is due to the fact that go/loader requires
// source files to have a .go filename extension, so using a more obvious name
// like "-" or even os.DevNull or os.Stdin.Name does not currently work.
//
// An EditedFileSystem can "pretend" that a file with this name exists in the
// current directory; the absolute path to this file is returned by
// FakeStdinPath.
const FakeStdinFilename = "-.go"

// FakeStdinPath returns the absolute path of a (likely nonexistent) file in
// the current directory whose name is given by FakeStdinFilename.
func FakeStdinPath() (string, error) {
	result, err := filepath.Abs(FakeStdinFilename)
	if err != nil {
		return FakeStdinFilename, err
	}
	return result, nil
}

/* -=-=- File System Interface -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// A FileSystem provides the ability to read directories and files, as well as
// to create, rename, and remove files (if the file system is not read-only).
type FileSystem interface {
	// ReadDir returns a slice of os.FileInfo, sorted by Name,
	// describing the content of the named directory.
	ReadDir(dir string) ([]os.FileInfo, error)

	// OpenFile opens a file (not a directory) for reading.
	OpenFile(path string) (io.ReadCloser, error)

	// OverwriteFile opens a file for writing.  It is an error if the
	// file does not already exist.
	OverwriteFile(path string) (io.WriteCloser, error)

	// CreateFile creates a text file with the given contents and default
	// permissions.
	CreateFile(path, contents string) error

	// Rename changes the name of a file or directory.  newName should be a
	// bare name, not including a directory prefix; the existing file will
	// be renamed within its existing parent directory.
	Rename(path, newName string) error

	// Remove deletes a file or an empty directory.
	Remove(path string) error
}

/* -=-=- Local File System -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// LocalFileSystem implements the FileSystem interface and provides access to
// the local file system by delegating to the os and io/ioutil packages.
type LocalFileSystem struct{}

func NewLocalFileSystem() *LocalFileSystem {
	return &LocalFileSystem{}
}

func (fs *LocalFileSystem) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}

func (fs *LocalFileSystem) OpenFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (fs *LocalFileSystem) OverwriteFile(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0666)
}

func (fs *LocalFileSystem) CreateFile(path, contents string) error {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return fmt.Errorf("Path already exists: %s", path)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(file, contents); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return nil
}

func (fs *LocalFileSystem) Rename(oldPath, newName string) error {
	if !isBareFilename(newName) {
		return fmt.Errorf("newName must be a bare filename: %s",
			newName)
	}
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	return os.Rename(oldPath, newPath)
}

func isBareFilename(filePath string) bool {
	dir, _ := filepath.Split(filePath)
	return dir == ""
}

func (fs *LocalFileSystem) Remove(path string) error {
	return os.Remove(path)
}

/* -=-=- Edited File System -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

type fileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string       { return fi.name }
func (fi *fileInfo) Size() int64        { return fi.size }
func (fi *fileInfo) Mode() os.FileMode  { return fi.mode }
func (fi *fileInfo) ModTime() time.Time { return fi.modTime }
func (fi *fileInfo) IsDir() bool        { return fi.isDir }
func (fi *fileInfo) Sys() interface{}   { return nil }

// EditedFileSystem implements the FileSystem interface and provides access to
// a hypothetical version of the local file system after a refactoring's
// changes have been applied.  This can be supplied to go/loader to analyze a
// program after refactoring, without actually changing the program on disk.
// File/directory creation, renaming, and deletion are not currently supported.
type EditedFileSystem struct {
	BaseFS FileSystem
	Edits  map[string]*text.EditSet
}

func NewEditedFileSystem(base FileSystem, edits map[string]*text.EditSet) *EditedFileSystem {
	return &EditedFileSystem{BaseFS: base, Edits: edits}
}

func NewSingleEditedFileSystem(filename, contents string) (*EditedFileSystem, error) {
	size, err := sizeOf(filename)
	if err != nil {
		return nil, err
	}
	es := text.NewEditSet()
	es.Add(&text.Extent{0, size}, contents)
	return NewEditedFileSystem(NewLocalFileSystem(), map[string]*text.EditSet{filename: es}), nil
}

func sizeOf(filename string) (int, error) {
	stdin, err := FakeStdinPath()
	if err != nil {
		return 0, err
	}
	if filename == stdin {
		return 0, nil
	}

	f, err := os.Open(filename)
	if err != nil {
		//if os.IsNotExist(err) {
		//	return 0, nil
		//}
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, err
	}
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	size := int(fi.Size())
	if int64(size) != fi.Size() {
		return 0, fmt.Errorf("File too large: %d bytes\n", fi.Size())
	}
	return size, nil
}

func (fs *EditedFileSystem) OpenFile(path string) (io.ReadCloser, error) {
	var localReader io.ReadCloser
	stdin, err := FakeStdinPath()
	if err != nil {
		return nil, err
	} else {
		localReader, err = fs.BaseFS.OpenFile(path)
		if err != nil && os.IsNotExist(err) && path == stdin {
			localReader = ioutil.NopCloser(strings.NewReader(""))
		} else if err != nil {
			return nil, err
		}
	}
	editSet, ok := fs.Edits[path]
	if !ok {
		return localReader, nil
	}
	contents, err := text.ApplyToReader(editSet, localReader)
	if err != nil {
		localReader.Close()
		return nil, err
	}
	if err := localReader.Close(); err != nil {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(contents)), nil
}

func (fs *EditedFileSystem) OverwriteFile(path string) (io.WriteCloser, error) {
	stdin, err := FakeStdinPath()
	if err != nil {
		return nil, err
	}
	if path == stdin {
		return os.Stdout, nil
	}

	return nil, fmt.Errorf("Cannot overwrite %s (EditedFileSystem)", path)
}

func (fs *EditedFileSystem) ReadDir(dirPath string) ([]os.FileInfo, error) {
	origInfos, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	result := []os.FileInfo{}
	for _, fi := range origInfos {
		filePath := filepath.Join(dirPath, fi.Name())
		if editSet, ok := fs.Edits[filePath]; !ok {
			result = append(result, fi)
		} else {
			newFileInfo := fileInfo{
				name:    fi.Name(),
				size:    fi.Size() + editSet.SizeChange(),
				mode:    fi.Mode(),
				modTime: fi.ModTime(),
				isDir:   fi.IsDir(),
			}
			result = append(result, &newFileInfo)
		}
	}

	stdin, err := FakeStdinPath()
	if err != nil {
		return result, err
	}
	if editSet, ok := fs.Edits[stdin]; ok {
		cwd, err := os.Getwd()
		if err != nil {
			return result, err
		}
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			return result, err
		}
		absPath, err := filepath.Abs(dirPath)
		if err != nil {
			return result, err
		}
		if absPath == absCwd {
			newFileInfo := fileInfo{
				name:    filepath.Base(stdin),
				size:    editSet.SizeChange(),
				mode:    0777,
				modTime: time.Now(),
				isDir:   false,
			}
			result = append(result, &newFileInfo)
		}
	}
	return result, nil
}

func (fs *EditedFileSystem) CreateFile(path, contents string) error {
	panic("CreateFile unsupported")
}

func (fs *EditedFileSystem) CreateDirectory(path string) error {
	panic("CreateDirectory unsupported")
}

func (fs *EditedFileSystem) Rename(path, newName string) error {
	panic("Rename unsupported")
}

func (fs *EditedFileSystem) Remove(path string) error {
	panic("Remove unsupported")
}

/* -=-=- Utility Functions -=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=- */

// CreatePatch reads bytes from a file, applying the edits in an EditSet and
// returning a Patch.
func CreatePatch(es *text.EditSet, fs FileSystem, filename string) (*text.Patch, error) {
	file, err := fs.OpenFile(filename)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return es.CreatePatch(file)
}

// ApplyEdits reads bytes from a file, applying the edits in an EditSet and
// returning the result as a slice of bytes.
func ApplyEdits(es *text.EditSet, fs FileSystem, filename string) ([]byte, error) {
	file, err := fs.OpenFile(filename)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	return text.ApplyToReader(es, file)
}
