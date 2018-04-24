// Copyright 2018 The etcd Authors
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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewRaftLogger(t *testing.T) {
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("test-log-%d", time.Now().UnixNano()))
	defer os.RemoveAll(logPath)

	lcfg := &zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{logPath},
		ErrorOutputPaths: []string{logPath},
	}
	gl, err := NewRaftLogger(lcfg)
	if err != nil {
		t.Fatal(err)
	}

	gl.Info("etcd-logutil-1")
	data, err := ioutil.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("etcd-logutil-1")) {
		t.Fatalf("can't find data in log %q", string(data))
	}

	gl.Warning("etcd-logutil-2")
	data, err = ioutil.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("etcd-logutil-2")) {
		t.Fatalf("can't find data in log %q", string(data))
	}
	if !bytes.Contains(data, []byte("logutil/zap_raft_test.go:")) {
		t.Fatalf("unexpected caller; %q", string(data))
	}
}

func TestNewRaftLoggerFromZapCore(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	syncer := zapcore.AddSync(buf)
	cr := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		syncer,
		zap.NewAtomicLevelAt(zap.InfoLevel),
	)

	lg := NewRaftLoggerFromZapCore(cr, syncer)
	lg.Info("TestNewRaftLoggerFromZapCore")
	txt := buf.String()
	if !strings.Contains(txt, "TestNewRaftLoggerFromZapCore") {
		t.Fatalf("unexpected log %q", txt)
	}
}
