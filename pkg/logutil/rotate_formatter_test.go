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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/fileutil"
	"github.com/coreos/pkg/capnslog"
)

func TestGetLogName(t *testing.T) {
	name := getLogName()
	if len(name) != 26 {
		t.Fatalf("unexpected %q", name)
	}
}

func TestRotateByFileSize(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "logtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64 = 100
		rotateDuration       = time.Duration(0)
	)
	ft, err := NewRotateFormatter(RotateConfig{
		Dir:            dir,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		t.Fatal(err)
	}

	capnslog.SetFormatter(ft)

	logger := capnslog.NewPackageLogger("test", "")

	logger.Println(strings.Repeat("a", 101))
	logger.Println("Hello World!")
	logger.Println("Hey!")

	fs, err := fileutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("expected 2 files, got %q", fs)
	}

	bts, err := ioutil.ReadFile(filepath.Join(dir, fs[1]))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(bts), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %q", lines)
	}
	if !strings.HasSuffix(lines[1], "Hey!") {
		t.Fatalf("unexpected line %q", lines[1])
	}
}

func TestRotateByDuration(t *testing.T) {
	dir, err := ioutil.TempDir(os.TempDir(), "logtest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		rotateFileSize int64
		rotateDuration = 50 * time.Millisecond
	)
	ft, err := NewRotateFormatter(RotateConfig{
		Dir:            dir,
		RotateFileSize: rotateFileSize,
		RotateDuration: rotateDuration,
	})
	if err != nil {
		t.Fatal(err)
	}

	capnslog.SetFormatter(ft)

	logger := capnslog.NewPackageLogger("test", "")

	time.Sleep(rotateDuration)
	logger.Println("Hello World!")
	logger.Println("Hello World!")
	logger.Println("Hey!")

	fs, err := fileutil.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("expected 2 files, got %q", fs)
	}

	bts, err := ioutil.ReadFile(filepath.Join(dir, fs[1]))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(bts), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %q", lines)
	}
	if !strings.HasSuffix(lines[1], "Hey!") {
		t.Fatalf("unexpected line %q", lines[1])
	}
}
