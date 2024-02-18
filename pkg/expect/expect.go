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

// Package expect implements a small expect-style interface
// TODO(ptab): Consider migration to https://github.com/google/goexpect.
package expect

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const DEBUG_LINES_TAIL = 40

type ExpectProcess struct {
	cmd  *exec.Cmd
	fpty *os.File
	wg   sync.WaitGroup

	mu    sync.Mutex // protects lines and err
	lines []string
	count int // increment whenever new line gets added
	err   error

	// StopSignal is the signal Stop sends to the process; defaults to SIGKILL.
	StopSignal os.Signal
}

// NewExpect creates a new process for expect testing.
func NewExpect(name string, arg ...string) (ep *ExpectProcess, err error) {
	// if env[] is nil, use current system env
	return NewExpectWithEnv(name, arg, nil)
}

// NewExpectWithEnv creates a new process with user defined env variables for expect testing.
func NewExpectWithEnv(name string, args []string, env []string) (ep *ExpectProcess, err error) {
	cmd := exec.Command(name, args...)
	cmd.Env = env
	ep = &ExpectProcess{
		cmd:        cmd,
		StopSignal: syscall.SIGKILL,
	}
	ep.cmd.Stderr = ep.cmd.Stdout
	ep.cmd.Stdin = nil

	if ep.fpty, err = pty.Start(ep.cmd); err != nil {
		return nil, err
	}

	ep.wg.Add(1)
	go ep.read()
	return ep, nil
}

func (ep *ExpectProcess) read() {
	defer ep.wg.Done()
	printDebugLines := os.Getenv("EXPECT_DEBUG") != ""
	r := bufio.NewReader(ep.fpty)
	for {
		l, err := r.ReadString('\n')
		ep.mu.Lock()
		if l != "" {
			if printDebugLines {
				fmt.Printf("%s-%d: %s", ep.cmd.Path, ep.cmd.Process.Pid, l)
			}
			ep.lines = append(ep.lines, l)
			ep.count++
		}
		if err != nil {
			ep.err = err
			ep.mu.Unlock()
			break
		}
		ep.mu.Unlock()
	}
}

// ExpectFunc returns the first line satisfying the function f.
func (ep *ExpectProcess) ExpectFunc(f func(string) bool) (string, error) {
	i := 0

	for {
		ep.mu.Lock()
		for i < len(ep.lines) {
			line := ep.lines[i]
			i++
			if f(line) {
				ep.mu.Unlock()
				return line, nil
			}
		}
		if ep.err != nil {
			ep.mu.Unlock()
			break
		}
		ep.mu.Unlock()
		time.Sleep(time.Millisecond * 100)
	}
	ep.mu.Lock()
	lastLinesIndex := len(ep.lines) - DEBUG_LINES_TAIL
	if lastLinesIndex < 0 {
		lastLinesIndex = 0
	}
	lastLines := strings.Join(ep.lines[lastLinesIndex:], "")
	ep.mu.Unlock()
	return "", fmt.Errorf("match not found."+
		" Set EXPECT_DEBUG for more info Err: %v, last lines:\n%s",
		ep.err, lastLines)
}

// Expect returns the first line containing the given string.
func (ep *ExpectProcess) Expect(s string) (string, error) {
	return ep.ExpectFunc(func(txt string) bool { return strings.Contains(txt, s) })
}

// LineCount returns the number of recorded lines since
// the beginning of the process.
func (ep *ExpectProcess) LineCount() int {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.count
}

// Stop kills the expect process and waits for it to exit.
func (ep *ExpectProcess) Stop() error { return ep.close(true) }

// Signal sends a signal to the expect process
func (ep *ExpectProcess) Signal(sig os.Signal) error {
	return ep.cmd.Process.Signal(sig)
}

// Wait waits for the process to finish.
func (ep *ExpectProcess) Wait() {
	ep.wg.Wait()
}

// Close waits for the expect process to exit.
// Close currently does not return error if process exited with !=0 status.
// TODO: Close should expose underlying proces failure by default.
func (ep *ExpectProcess) Close() error { return ep.close(false) }

func (ep *ExpectProcess) close(kill bool) error {
	if ep.cmd == nil {
		return ep.err
	}
	if kill {
		ep.Signal(ep.StopSignal)
	}

	err := ep.cmd.Wait()
	ep.fpty.Close()
	ep.wg.Wait()

	if err != nil {
		if !kill && strings.Contains(err.Error(), "exit status") {
			// non-zero exit code
			err = nil
		} else if kill && strings.Contains(err.Error(), "signal:") {
			err = nil
		}
	}

	ep.cmd = nil
	return err
}

func (ep *ExpectProcess) Send(command string) error {
	_, err := io.WriteString(ep.fpty, command)
	return err
}

func (ep *ExpectProcess) ProcessError() error {
	if strings.Contains(ep.err.Error(), "input/output error") {
		// TODO: The expect library should not return
		// `/dev/ptmx: input/output error` when process just exits.
		return nil
	}
	return ep.err
}

func (ep *ExpectProcess) Lines() []string {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.lines
}
