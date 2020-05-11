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

package zapcore_test

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"go.uber.org/zap/internal/ztest"
	. "go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeInt64Field(key string, val int) Field {
	return Field{Type: Int64Type, Integer: int64(val), Key: key}
}

func TestNopCore(t *testing.T) {
	entry := Entry{
		Message:    "test",
		Level:      InfoLevel,
		Time:       time.Now(),
		LoggerName: "main",
		Stack:      "fake-stack",
	}
	ce := &CheckedEntry{}

	allLevels := []Level{
		DebugLevel,
		InfoLevel,
		WarnLevel,
		ErrorLevel,
		DPanicLevel,
		PanicLevel,
		FatalLevel,
	}
	core := NewNopCore()
	assert.Equal(t, core, core.With([]Field{makeInt64Field("k", 42)}), "Expected no-op With.")
	for _, level := range allLevels {
		assert.False(t, core.Enabled(level), "Expected all levels to be disabled in no-op core.")
		assert.Equal(t, ce, core.Check(entry, ce), "Expected no-op Check to return checked entry unchanged.")
		assert.NoError(t, core.Write(entry, nil), "Expected no-op Writes to always succeed.")
		assert.NoError(t, core.Sync(), "Expected no-op Syncs to always succeed.")
	}
}

func TestIOCore(t *testing.T) {
	temp, err := ioutil.TempFile("", "zapcore-test-iocore")
	require.NoError(t, err, "Failed to create temp file.")
	defer os.Remove(temp.Name())

	// Drop timestamps for simpler assertions (timestamp encoding is tested
	// elsewhere).
	cfg := testEncoderConfig()
	cfg.TimeKey = ""

	core := NewCore(
		NewJSONEncoder(cfg),
		temp,
		InfoLevel,
	).With([]Field{makeInt64Field("k", 1)})
	defer assert.NoError(t, core.Sync(), "Expected Syncing a temp file to succeed.")

	if ce := core.Check(Entry{Level: DebugLevel, Message: "debug"}, nil); ce != nil {
		ce.Write(makeInt64Field("k", 2))
	}
	if ce := core.Check(Entry{Level: InfoLevel, Message: "info"}, nil); ce != nil {
		ce.Write(makeInt64Field("k", 3))
	}
	if ce := core.Check(Entry{Level: WarnLevel, Message: "warn"}, nil); ce != nil {
		ce.Write(makeInt64Field("k", 4))
	}

	logged, err := ioutil.ReadFile(temp.Name())
	require.NoError(t, err, "Failed to read from temp file.")
	require.Equal(
		t,
		`{"level":"info","msg":"info","k":1,"k":3}`+"\n"+
			`{"level":"warn","msg":"warn","k":1,"k":4}`+"\n",
		string(logged),
		"Unexpected log output.",
	)
}

func TestIOCoreSyncFail(t *testing.T) {
	sink := &ztest.Discarder{}
	err := errors.New("failed")
	sink.SetError(err)

	core := NewCore(
		NewJSONEncoder(testEncoderConfig()),
		sink,
		DebugLevel,
	)

	assert.Equal(
		t,
		err,
		core.Sync(),
		"Expected core.Sync to return errors from underlying WriteSyncer.",
	)
}

func TestIOCoreSyncsOutput(t *testing.T) {
	tests := []struct {
		entry      Entry
		shouldSync bool
	}{
		{Entry{Level: DebugLevel}, false},
		{Entry{Level: InfoLevel}, false},
		{Entry{Level: WarnLevel}, false},
		{Entry{Level: ErrorLevel}, false},
		{Entry{Level: DPanicLevel}, true},
		{Entry{Level: PanicLevel}, true},
		{Entry{Level: FatalLevel}, true},
	}

	for _, tt := range tests {
		sink := &ztest.Discarder{}
		core := NewCore(
			NewJSONEncoder(testEncoderConfig()),
			sink,
			DebugLevel,
		)

		core.Write(tt.entry, nil)
		assert.Equal(t, tt.shouldSync, sink.Called(), "Incorrect Sync behavior.")
	}
}

func TestIOCoreWriteFailure(t *testing.T) {
	core := NewCore(
		NewJSONEncoder(testEncoderConfig()),
		Lock(&ztest.FailWriter{}),
		DebugLevel,
	)
	err := core.Write(Entry{}, nil)
	// Should log the error.
	assert.Error(t, err, "Expected writing Entry to fail.")
}
