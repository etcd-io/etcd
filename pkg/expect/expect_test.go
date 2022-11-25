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
	if err != nil {
		t.Fatal(err)
	}
	wstr := "hello world\r\n"
	l, eerr := ep.ExpectFunc(context.Background(), func(a string) bool { return len(a) > 10 })
	if eerr != nil {
		t.Fatal(eerr)
	}
	if l != wstr {
		t.Fatalf(`got "%v", expected "%v"`, l, wstr)
	}
	if cerr := ep.Close(); cerr != nil {
		t.Fatal(cerr)
	}
}

func TestExpectFuncTimeout(t *testing.T) {
	ep, err := NewExpect("tail", "-f", "/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		// It's enough to have "talkative" process to stuck in the infinite loop of reading
		for {
			err := ep.Send("new line\n")
			if err != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = ep.ExpectFunc(ctx, func(a string) bool { return false })

	require.ErrorAs(t, err, &context.DeadlineExceeded)

	if err := ep.Stop(); err != nil {
		t.Fatal(err)
	}

	err = ep.Close()
	require.ErrorContains(t, err, "unexpected exit code [-1] after running [/usr/bin/tail -f /dev/null]")
	require.Equal(t, -1, ep.exitCode)
}

func TestExpectFuncExitFailure(t *testing.T) {
	// tail -x should not exist and return a non-zero exit code
	ep, err := NewExpect("tail", "-x")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = ep.ExpectFunc(ctx, func(s string) bool {
		return strings.Contains(s, "something entirely unexpected")
	})
	require.ErrorContains(t, err, "unexpected exit code [1] after running [/usr/bin/tail -x]")
	require.Equal(t, 1, ep.exitCode)
}

func TestExpectFuncExitFailureStop(t *testing.T) {
	// tail -x should not exist and return a non-zero exit code
	ep, err := NewExpect("tail", "-x")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = ep.ExpectFunc(ctx, func(s string) bool {
		return strings.Contains(s, "something entirely unexpected")
	})
	require.ErrorContains(t, err, "unexpected exit code [1] after running [/usr/bin/tail -x]")
	exitCode, err := ep.ExitCode()
	require.Equal(t, 0, exitCode)
	require.Equal(t, err, ErrProcessRunning)
	if err := ep.Stop(); err != nil {
		t.Fatal(err)
	}
	err = ep.Close()
	require.ErrorContains(t, err, "unexpected exit code [1] after running [/usr/bin/tail -x]")
	exitCode, err = ep.ExitCode()
	require.Equal(t, 1, exitCode)
	require.NoError(t, err)
}

func TestEcho(t *testing.T) {
	ep, err := NewExpect("echo", "hello world")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	l, eerr := ep.ExpectWithContext(ctx, "world")
	if eerr != nil {
		t.Fatal(eerr)
	}
	wstr := "hello world"
	if l[:len(wstr)] != wstr {
		t.Fatalf(`got "%v", expected "%v"`, l, wstr)
	}
	if cerr := ep.Close(); cerr != nil {
		t.Fatal(cerr)
	}
	if _, eerr = ep.ExpectWithContext(ctx, "..."); eerr == nil {
		t.Fatalf("expected error on closed expect process")
	}
}

func TestLineCount(t *testing.T) {
	ep, err := NewExpect("printf", "1\n2\n3")
	if err != nil {
		t.Fatal(err)
	}
	wstr := "3"
	l, eerr := ep.ExpectWithContext(context.Background(), wstr)
	if eerr != nil {
		t.Fatal(eerr)
	}
	if l != wstr {
		t.Fatalf(`got "%v", expected "%v"`, l, wstr)
	}
	if ep.LineCount() != 3 {
		t.Fatalf("got %d, expected 3", ep.LineCount())
	}
	if cerr := ep.Close(); cerr != nil {
		t.Fatal(cerr)
	}
}

func TestSend(t *testing.T) {
	ep, err := NewExpect("tr", "a", "b")
	if err != nil {
		t.Fatal(err)
	}
	if err := ep.Send("a\r"); err != nil {
		t.Fatal(err)
	}
	if _, err := ep.ExpectWithContext(context.Background(), "b"); err != nil {
		t.Fatal(err)
	}
	if err := ep.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestSignal(t *testing.T) {
	ep, err := NewExpect("sleep", "100")
	if err != nil {
		t.Fatal(err)
	}
	ep.Signal(os.Interrupt)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		err = ep.Close()
		assert.ErrorContains(t, err, "unexpected exit code [-1]")
		assert.ErrorContains(t, err, "sleep 100")
	}()
	select {
	case <-time.After(5 * time.Second):
		t.Fatalf("signal test timed out")
	case <-donec:
	}
}
