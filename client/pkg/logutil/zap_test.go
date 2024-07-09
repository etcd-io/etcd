// Copyright 2024 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logutil

import (
	"bytes"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type commonLogFields struct {
	Level     string `json:"level"`
	Timestamp string `json:"ts"`
	Message   string `json:"msg"`
}

const (
	fractionSecondsPrecision = 6 // MicroSeconds
)

func TestEncodeTimePrecisionToMicroSeconds(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	syncer := zapcore.AddSync(buf)
	zc := zapcore.NewCore(
		zapcore.NewJSONEncoder(DefaultZapLoggerConfig.EncoderConfig),
		syncer,
		zap.NewAtomicLevelAt(zap.InfoLevel),
	)

	lg := zap.New(zc)
	lg.Info("TestZapLog")
	fields := commonLogFields{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &fields))
	// example 1: 2024-06-06T23:37:21.948385Z
	// example 2 with zone offset: 2024-06-06T16:16:44.176778-0700
	regex := `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.(\d+)(Z|[+-]\d{4})`
	re := regexp.MustCompile(regex)
	matches := re.FindStringSubmatch(fields.Timestamp)
	require.Equal(t, 3, len(matches))
	require.Equalf(t, fractionSecondsPrecision, len(matches[1]), "unexpected timestamp %s", fields.Timestamp)
}
