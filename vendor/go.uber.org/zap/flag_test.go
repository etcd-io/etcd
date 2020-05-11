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
	"flag"
	"io/ioutil"
	"testing"

	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
)

type flagTestCase struct {
	args      []string
	wantLevel zapcore.Level
	wantErr   bool
}

func (tc flagTestCase) runImplicitSet(t testing.TB) {
	origCommandLine := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(ioutil.Discard)
	defer func() { flag.CommandLine = origCommandLine }()

	level := LevelFlag("level", InfoLevel, "")
	tc.run(t, flag.CommandLine, level)
}

func (tc flagTestCase) runExplicitSet(t testing.TB) {
	var lvl zapcore.Level
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.Var(&lvl, "level", "minimum enabled logging level")
	tc.run(t, set, &lvl)
}

func (tc flagTestCase) run(t testing.TB, set *flag.FlagSet, actual *zapcore.Level) {
	err := set.Parse(tc.args)
	if tc.wantErr {
		assert.Error(t, err, "Parse(%v) should fail.", tc.args)
		return
	}
	if assert.NoError(t, err, "Parse(%v) should succeed.", tc.args) {
		assert.Equal(t, tc.wantLevel, *actual, "Level mismatch.")
	}
}

func TestLevelFlag(t *testing.T) {
	tests := []flagTestCase{
		{
			args:      nil,
			wantLevel: zapcore.InfoLevel,
		},
		{
			args:    []string{"--level", "unknown"},
			wantErr: true,
		},
		{
			args:      []string{"--level", "error"},
			wantLevel: zapcore.ErrorLevel,
		},
	}

	for _, tt := range tests {
		tt.runExplicitSet(t)
		tt.runImplicitSet(t)
	}
}

func TestLevelFlagsAreIndependent(t *testing.T) {
	origCommandLine := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(ioutil.Discard)
	defer func() { flag.CommandLine = origCommandLine }()

	// Make sure that these two flags are independent.
	fileLevel := LevelFlag("file-level", InfoLevel, "")
	consoleLevel := LevelFlag("console-level", InfoLevel, "")

	assert.NoError(t, flag.CommandLine.Parse([]string{"-file-level", "debug"}), "Unexpected flag-parsing error.")
	assert.Equal(t, InfoLevel, *consoleLevel, "Expected file logging level to remain unchanged.")
	assert.Equal(t, DebugLevel, *fileLevel, "Expected console logging level to have changed.")
}
