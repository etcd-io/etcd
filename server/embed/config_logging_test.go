// Copyright 2025 The etcd Authors
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

package embed

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

type mockTLSConn struct {
	state      tls.ConnectionState
	remoteAddr net.Addr
}

func (m *mockTLSConn) ConnectionState() tls.ConnectionState {
	return m.state
}

func (m *mockTLSConn) RemoteAddr() net.Addr {
	return m.remoteAddr
}

func TestConfig_SetupLogging_TLSHandshakeFailureFunc(t *testing.T) {
	conn := mockTLSConn{
		remoteAddr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 8000,
		},
	}

	testingHandshakeFailureTLSConn = &conn
	defer func() {
		testingHandshakeFailureTLSConn = nil
	}()

	testCases := []struct {
		name                   string
		err                    error
		expectedLogEntryPrefix string
	}{
		{
			name:                   "custom error",
			err:                    errors.New("nuked"),
			expectedLogEntryPrefix: "WARN",
		},
		{
			name:                   "EOF error",
			err:                    io.EOF,
			expectedLogEntryPrefix: "DEBUG",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var out zaptest.Buffer

			encoderConfig := zap.NewDevelopmentEncoderConfig()
			encoderConfig.EncodeTime = nil

			logger := zap.New(zapcore.NewCore(
				zapcore.NewConsoleEncoder(encoderConfig),
				&out,
				zapcore.DebugLevel,
			))

			c := NewConfig()
			c.ZapLoggerBuilder = NewZapLoggerBuilder(logger)
			if err := c.Validate(); err != nil {
				t.Fatal("Failed to validate config:", err)
			}

			out.Reset()
			c.ClientTLSInfo.HandshakeFailure(nil, tc.err)

			got := out.Lines()
			if len(got) != 1 {
				t.Fatal("Expected 1 log entry, got log entries:", got)
			}

			line := got[0]
			if !strings.HasPrefix(line, tc.expectedLogEntryPrefix) {
				t.Errorf("Expected log entry prefix %q, got log entry: %s", tc.expectedLogEntryPrefix, got[0])
			}
		})
	}
}
