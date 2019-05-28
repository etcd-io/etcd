// Copyright 2017 The etcd Authors
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

package logutil_test

import (
	"bytes"
	"io/ioutil"
	"strings"
	"testing"

	"go.etcd.io/etcd/pkg/logutil"

	"github.com/coreos/pkg/capnslog"
)

func TestPackageLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	capnslog.SetFormatter(capnslog.NewDefaultFormatter(buf))

	l := logutil.NewPackageLogger("go.etcd.io/etcd", "logger")

	r := capnslog.MustRepoLogger("go.etcd.io/etcd")
	r.SetLogLevel(map[string]capnslog.LogLevel{"logger": capnslog.INFO})

	l.Infof("hello world!")
	if !strings.Contains(buf.String(), "hello world!") {
		t.Fatalf("expected 'hello world!', got %q", buf.String())
	}
	buf.Reset()

	// capnslog.INFO is 3
	l.Lvl(2).Infof("Level 2")
	l.Lvl(5).Infof("Level 5")
	if !strings.Contains(buf.String(), "Level 2") {
		t.Fatalf("expected 'Level 2', got %q", buf.String())
	}
	if strings.Contains(buf.String(), "Level 5") {
		t.Fatalf("unexpected 'Level 5', got %q", buf.String())
	}
	buf.Reset()

	capnslog.SetFormatter(capnslog.NewDefaultFormatter(ioutil.Discard))
	l.Infof("ignore this")
	if len(buf.Bytes()) > 0 {
		t.Fatalf("unexpected logs %q", buf.String())
	}
}
