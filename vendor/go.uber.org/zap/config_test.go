// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package zap

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		desc     string
		cfg      Config
		expectN  int64
		expectRe string
	}{
		{
			desc:    "production",
			cfg:     NewProductionConfig(),
			expectN: 2 + 100 + 1, // 2 from initial logs, 100 initial sampled logs, 1 from off-by-one in sampler
			expectRe: `{"level":"info","caller":"zap/config_test.go:\d+","msg":"info","k":"v","z":"zz"}` + "\n" +
				`{"level":"warn","caller":"zap/config_test.go:\d+","msg":"warn","k":"v","z":"zz"}` + "\n",
		},
		{
			desc:    "development",
			cfg:     NewDevelopmentConfig(),
			expectN: 3 + 200, // 3 initial logs, all 200 subsequent logs
			expectRe: "DEBUG\tzap/config_test.go:" + `\d+` + "\tdebug\t" + `{"k": "v", "z": "zz"}` + "\n" +
				"INFO\tzap/config_test.go:" + `\d+` + "\tinfo\t" + `{"k": "v", "z": "zz"}` + "\n" +
				"WARN\tzap/config_test.go:" + `\d+` + "\twarn\t" + `{"k": "v", "z": "zz"}` + "\n" +
				`testing.\w+`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			temp, err := ioutil.TempFile("", "zap-prod-config-test")
			require.NoError(t, err, "Failed to create temp file.")
			defer os.Remove(temp.Name())

			tt.cfg.OutputPaths = []string{temp.Name()}
			tt.cfg.EncoderConfig.TimeKey = "" // no timestamps in tests
			tt.cfg.InitialFields = map[string]interface{}{"z": "zz", "k": "v"}

			hook, count := makeCountingHook()
			logger, err := tt.cfg.Build(Hooks(hook))
			require.NoError(t, err, "Unexpected error constructing logger.")

			logger.Debug("debug")
			logger.Info("info")
			logger.Warn("warn")

			byteContents, err := ioutil.ReadAll(temp)
			require.NoError(t, err, "Couldn't read log contents from temp file.")
			logs := string(byteContents)
			assert.Regexp(t, tt.expectRe, logs, "Unexpected log output.")

			for i := 0; i < 200; i++ {
				logger.Info("sampling")
			}
			assert.Equal(t, tt.expectN, count.Load(), "Hook called an unexpected number of times.")
		})
	}
}

func TestConfigWithInvalidPaths(t *testing.T) {
	tests := []struct {
		desc      string
		output    string
		errOutput string
	}{
		{"output directory doesn't exist", "/tmp/not-there/foo.log", "stderr"},
		{"error output directory doesn't exist", "stdout", "/tmp/not-there/foo-errors.log"},
		{"neither output directory exists", "/tmp/not-there/foo.log", "/tmp/not-there/foo-errors.log"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := NewProductionConfig()
			cfg.OutputPaths = []string{tt.output}
			cfg.ErrorOutputPaths = []string{tt.errOutput}
			_, err := cfg.Build()
			assert.Error(t, err, "Expected an error opening a non-existent directory.")
		})
	}
}
