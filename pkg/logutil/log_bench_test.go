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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/pkg/capnslog"
)

func Benchmark_capnslog_rotate_without_flock(b *testing.B) {
	dir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64 = 3 * 1024 // 3KB
		rotateDuration       = time.Duration(0)
	)
	ft, err := NewRotateFormatter(Config{
		Dir:            dir,
		FileLock:       false,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		b.Fatal(err)
	}

	capnslog.SetFormatter(ft)

	logger := capnslog.NewPackageLogger("test", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Println("TEST")
	}
}

func Benchmark_capnslog_rotate_with_flock(b *testing.B) {
	dir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64 = 3 * 1024 // 3KB
		rotateDuration       = time.Duration(0)
	)
	ft, err := NewRotateFormatter(Config{
		Dir:            dir,
		FileLock:       true,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		b.Fatal(err)
	}

	capnslog.SetFormatter(ft)

	logger := capnslog.NewPackageLogger("test", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Println("TEST")
	}
}

func Benchmark_capnslog(b *testing.B) {
	dir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	fpath := filepath.Join(dir, "test.log")

	f, err := openToAppendOnly(fpath)
	if err != nil {
		b.Fatal(err)
	}

	capnslog.SetFormatter(capnslog.NewPrettyFormatter(f, true))

	logger := capnslog.NewPackageLogger("test", "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Println("TEST")
	}

	if err = f.Close(); err != nil {
		b.Fatal(err)
	}
}

func Benchmark_log(b *testing.B) {
	dir, err := ioutil.TempDir(os.TempDir(), "log_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)

	fpath := filepath.Join(dir, "test.log")

	f, err := openToAppendOnly(fpath)
	if err != nil {
		b.Fatal(err)
	}

	logger := log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Println("TEST")
	}

	if err = f.Close(); err != nil {
		b.Fatal(err)
	}
}

func openToAppendOnly(fpath string) (*os.File, error) {
	f, err := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	return f, nil
}
