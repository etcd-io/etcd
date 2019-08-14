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

	"github.com/coreos/etcd/pkg/logutil"

	"google.golang.org/grpc/grpclog"
)

func TestLogger(t *testing.T) {
	buf := new(bytes.Buffer)

	l := logutil.NewLogger(grpclog.NewLoggerV2WithVerbosity(buf, buf, buf, 10))
	l.Infof("hello world!")
	if !strings.Contains(buf.String(), "hello world!") {
		t.Fatalf("expected 'hello world!', got %q", buf.String())
	}
	buf.Reset()

	l.Lvl(10).Infof("Level 10")
	l.Lvl(30).Infof("Level 30")
	if !strings.Contains(buf.String(), "Level 10") {
		t.Fatalf("expected 'Level 10', got %q", buf.String())
	}
	if strings.Contains(buf.String(), "Level 30") {
		t.Fatalf("unexpected 'Level 30', got %q", buf.String())
	}
	buf.Reset()

	l = logutil.NewLogger(grpclog.NewLoggerV2(ioutil.Discard, ioutil.Discard, ioutil.Discard))
	l.Infof("ignore this")
	if len(buf.Bytes()) > 0 {
		t.Fatalf("unexpected logs %q", buf.String())
	}
}
