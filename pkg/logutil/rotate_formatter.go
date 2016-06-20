// Copyright 2016 The etcd Authors
//
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

package logutil

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/pkg/capnslog"
)

var numRegex = regexp.MustCompile("[0-9]+")

// getLogName generates a log file name based on current time
// (e.g. 20160619-142321-397035.log).
func getLogName() string {
	txt := time.Now().String()[:26]
	txt = strings.Join(numRegex.FindAllString(txt, -1), "")
	return txt[:8] + "-" + txt[8:14] + "-" + txt[14:] + ".log"
}

// Config contains configuration for log rotation.
type Config struct {
	// Dir is the directory to put log files.
	Dir      string
	Debug    bool
	FileLock bool

	RotateFileSize int64
	RotateDuration time.Duration
}

type formatter struct {
	dir   string
	debug bool

	rotateFileSize int64

	rotateDuration time.Duration
	started        time.Time

	w *bufio.Writer

	fileLock bool
	file     *fileutil.LockedFile
}

// NewRotateFormatter returns a new formatter.
func NewRotateFormatter(cfg Config) (capnslog.Formatter, error) {
	if err := fileutil.TouchDirAll(cfg.Dir); err != nil {
		return nil, err
	}

	// create temporary directory, and rename later to make it appear atomic
	tmpDir := filepath.Clean(cfg.Dir) + ".tmp"
	if fileutil.Exist(tmpDir) {
		if err := os.RemoveAll(tmpDir); err != nil {
			return nil, err
		}
	}
	if err := fileutil.TouchDirAll(tmpDir); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	var (
		logName    = getLogName()
		logPath    = filepath.Join(cfg.Dir, logName)
		logPathTmp = filepath.Join(tmpDir, logName)

		f   *fileutil.LockedFile
		err error
	)
	switch cfg.FileLock {
	case true:
		f, err = fileutil.LockFile(logPathTmp, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
		if err != nil {
			return nil, err
		}
	case false:
		var tFile *os.File
		tFile, err = os.OpenFile(logPathTmp, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
		if err != nil {
			return nil, err
		}
		f = &fileutil.LockedFile{tFile}
	}

	if err = os.Rename(logPathTmp, logPath); err != nil {
		return nil, err
	}

	ft := &formatter{
		dir:            cfg.Dir,
		debug:          cfg.Debug,
		rotateFileSize: cfg.RotateFileSize,
		rotateDuration: cfg.RotateDuration,
		started:        time.Now(),
		w:              bufio.NewWriter(f),
		fileLock:       cfg.FileLock,
		file:           f,
	}
	return ft, nil
}

// Format writes log and flushes it. This is protected by mutex in capnslog.
func (ft *formatter) Format(pkg string, lvl capnslog.LogLevel, depth int, entries ...interface{}) {
	if !ft.debug && lvl == capnslog.DEBUG {
		return
	}

	ft.w.WriteString(time.Now().String()[:26])
	ft.w.WriteString(" " + lvl.String() + " | ")
	if pkg != "" {
		ft.w.WriteString(pkg + ": ")
	}

	txt := fmt.Sprint(entries...)
	ft.w.WriteString(txt)

	if !strings.HasSuffix(txt, "\n") {
		ft.w.WriteString("\n")
	}

	// fsync to the disk
	ft.w.Flush()

	// seek the current location, and get the offset
	curOffset, err := ft.file.Seek(0, os.SEEK_CUR)
	if err != nil {
		panic(err)
	}

	var (
		needRotateBySize     = ft.rotateFileSize > 0 && curOffset > ft.rotateFileSize
		needRotateByDuration = ft.rotateDuration > time.Duration(0) && time.Since(ft.started) > ft.rotateDuration
	)
	if needRotateBySize || needRotateByDuration {
		ft.unsafeRotate()
	}
}

func (ft *formatter) unsafeRotate() {
	// unlock the locked file
	if err := ft.file.Close(); err != nil {
		panic(err)
	}

	var (
		logPath    = filepath.Join(ft.dir, getLogName())
		logPathTmp = logPath + ".tmp"

		fileTmp *fileutil.LockedFile
		err     error
	)
	switch ft.fileLock {
	case true:
		fileTmp, err = fileutil.LockFile(logPathTmp, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
		if err != nil {
			panic(err)
		}
	case false:
		var tFile *os.File
		tFile, err = os.OpenFile(logPathTmp, os.O_WRONLY|os.O_CREATE, fileutil.PrivateFileMode)
		if err != nil {
			panic(err)
		}
		fileTmp = &fileutil.LockedFile{tFile}
	}

	// rename the file to WAL name atomically
	if err = os.Rename(logPathTmp, logPath); err != nil {
		panic(err)
	}

	// release the lock, flush buffer
	if err = fileTmp.Close(); err != nil {
		panic(err)
	}

	// create a new locked file for appends
	switch ft.fileLock {
	case true:
		fileTmp, err = fileutil.LockFile(logPath, os.O_WRONLY, fileutil.PrivateFileMode)
		if err != nil {
			panic(err)
		}
	case false:
		var tFile *os.File
		tFile, err = os.OpenFile(logPath, os.O_WRONLY, fileutil.PrivateFileMode)
		if err != nil {
			panic(err)
		}
		fileTmp = &fileutil.LockedFile{tFile}
	}

	ft.w = bufio.NewWriter(fileTmp)
	ft.file = fileTmp

	ft.started = time.Now()
}

func (ft *formatter) Flush() {
	ft.w.Flush()
}
