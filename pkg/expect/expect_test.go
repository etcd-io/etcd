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

// build !windows

package expect

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpectFunc(t *testing.T) {
	ep, err := NewExpect("echo", "hello world")
	require.NoError(t, err)
	wstr := "hello world\r\n"
	l, eerr := ep.ExpectFunc(t.Context(), func(a string) bool { return len(a) > 10 })
	require.NoError(t, eerr)
	require.Equalf(t, l, wstr, `got "%v", expected "%v"`, l, wstr)
	require.NoError(t, ep.Close())
}

func TestExpectFuncTimeout(t *testing.T) {
	ep, err := NewExpect("tail", "-f", "/dev/null")
	require.NoError(t, err)
	go func() {
		// It's enough to have "talkative" process to stuck in the infinite loop of reading
		for {
			if serr := ep.Send("new line\n"); serr != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_, err = ep.ExpectFunc(ctx, func(a string) bool { return false })

	require.ErrorIs(t, err, context.DeadlineExceeded)

	require.NoError(t, ep.Stop())
	require.ErrorContains(t, ep.Close(), "unexpected exit code [143]")
	require.Equal(t, 143, ep.exitCode)
}

func TestExpectFuncExitFailure(t *testing.T) {
	// tail -x should not exist and return a non-zero exit code
	ep, err := NewExpect("tail", "-x")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_, err = ep.ExpectFunc(ctx, func(s string) bool {
		return strings.Contains(s, "something entirely unexpected")
	})
	require.ErrorContains(t, err, "unexpected exit code [1]")
	require.Equal(t, 1, ep.exitCode)
}

func TestExpectFuncExitFailureStop(t *testing.T) {
	// tail -x should not exist and return a non-zero exit code
	ep, err := NewExpect("tail", "-x")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	_, err = ep.ExpectFunc(ctx, func(s string) bool {
		return strings.Contains(s, "something entirely unexpected")
	})
	require.ErrorContains(t, err, "unexpected exit code [1]")
	exitCode, err := ep.ExitCode()
	require.Equal(t, 1, exitCode)
	require.NoError(t, err)
	require.NoError(t, ep.Stop())
	require.ErrorContains(t, ep.Close(), "unexpected exit code [1]")
	exitCode, err = ep.ExitCode()
	require.Equal(t, 1, exitCode)
	require.NoError(t, err)
}

func TestEcho(t *testing.T) {
	ep, err := NewExpect("echo", "hello world")
	require.NoError(t, err)
	ctx := t.Context()
	l, eerr := ep.ExpectWithContext(ctx, ExpectedResponse{Value: "world"})
	require.NoError(t, eerr)
	wstr := "hello world"
	require.Equalf(t, l[:len(wstr)], wstr, `got "%v", expected "%v"`, l, wstr)
	require.NoError(t, ep.Close())
	_, eerr = ep.ExpectWithContext(ctx, ExpectedResponse{Value: "..."})
	require.Errorf(t, eerr, "expected error on closed expect process")
}

func TestLineCount(t *testing.T) {
	ep, err := NewExpect("printf", "1\n2\n3")
	require.NoError(t, err)
	wstr := "3"
	l, eerr := ep.ExpectWithContext(t.Context(), ExpectedResponse{Value: wstr})
	require.NoError(t, eerr)
	require.Equalf(t, l, wstr, `got "%v", expected "%v"`, l, wstr)
	require.Equalf(t, 3, ep.LineCount(), "got %d, expected 3", ep.LineCount())
	require.NoError(t, ep.Close())
}

func TestSend(t *testing.T) {
	ep, err := NewExpect("tr", "a", "b")
	require.NoError(t, err)
	err = ep.Send("a\r")
	require.NoError(t, err)
	_, err = ep.ExpectWithContext(t.Context(), ExpectedResponse{Value: "b"})
	require.NoError(t, err)
	require.NoError(t, ep.Stop())
}

func TestSignal(t *testing.T) {
	ep, err := NewExpect("sleep", "100")
	require.NoError(t, err)
	ep.Signal(os.Interrupt)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		err = ep.Close()
		assert.ErrorContains(t, err, "unexpected exit code [130]")
		assert.ErrorContains(t, err, "sleep 100")
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("signal test timed out")
	case <-donec:
	}
}

func TestExitCodeAfterKill(t *testing.T) {
	ep, err := NewExpect("sleep", "100")
	require.NoError(t, err)

	ep.Signal(os.Kill)
	ep.Wait()
	code, err := ep.ExitCode()
	assert.Equal(t, 137, code)
	assert.NoError(t, err)
}

func TestExpectForFailFastCommand(t *testing.T) {
	ep, err := NewExpect("sh", "-c", `echo "curl: (59) failed setting cipher list"; exit 59`)
	require.NoError(t, err)

	_, err = ep.Expect("failed setting cipher list")
	require.NoError(t, err)
}

func TestResponseMatchRegularExpr(t *testing.T) {
	testCases := []struct {
		name         string
		mockOutput   string
		expectedResp ExpectedResponse
		expectMatch  bool
	}{
		{
			name:         "exact match",
			mockOutput:   "hello world",
			expectedResp: ExpectedResponse{Value: "hello world"},
			expectMatch:  true,
		},
		{
			name:         "not exact match",
			mockOutput:   "hello world",
			expectedResp: ExpectedResponse{Value: "hello wld"},
			expectMatch:  false,
		},
		{
			name:         "match regular expression",
			mockOutput:   "hello world",
			expectedResp: ExpectedResponse{Value: `.*llo\sworld`, IsRegularExpr: true},
			expectMatch:  true,
		},
		{
			name:         "not match regular expression",
			mockOutput:   "hello world",
			expectedResp: ExpectedResponse{Value: `.*llo wrld`, IsRegularExpr: true},
			expectMatch:  false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ep, err := NewExpect("echo", "-n", tc.mockOutput)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			l, err := ep.ExpectWithContext(ctx, tc.expectedResp)

			if tc.expectMatch {
				require.Equal(t, tc.mockOutput, l)
			} else {
				require.Error(t, err)
			}

			require.NoError(t, ep.Close())
		})
	}
}
