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

// +build !windows

package logutil

import (
	"bytes"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewJournalWriter(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	jw, err := NewJournalWriter(buf)
	if err != nil {
		t.Skip(err)
	}

	syncer := zapcore.AddSync(jw)

	cr := zapcore.NewCore(
		zapcore.NewJSONEncoder(DefaultZapLoggerConfig.EncoderConfig),
		syncer,
		zap.NewAtomicLevelAt(zap.InfoLevel),
	)

	lg := zap.New(cr, zap.AddCaller(), zap.ErrorOutput(syncer))
	defer lg.Sync()

	lg.Info("TestNewJournalWriter")
	if buf.String() == "" {
		// check with "journalctl -f"
		t.Log("sent logs successfully to journald")
	}
}
