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
package expect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const DEBUG_LINES_TAIL = 40

var (
	ErrProcessRunning = fmt.Errorf("process is still running")
)

type ExpectedResponse struct {
	Value         string
	IsRegularExpr bool
}

type ExpectProcess struct {
	cfg expectConfig

	cmd  *exec.Cmd
	fpty *os.File
	wg   sync.WaitGroup

	readCloseCh chan struct{} // close it if async read goroutine exits

	mu       sync.Mutex // protects lines, count, cur, exitErr and exitCode
	lines    []string
	count    int   // increment whenever new line gets added
	cur      int   // current read position
	exitErr  error // process exit error
	exitCode int
}

// NewExpect creates a new process for expect testing.
func NewExpect(name string, arg ...string) (ep *ExpectProcess, err error) {
	// if env[] is nil, use current system env and the default command as name
	return NewExpectWithEnv(name, arg, nil, name)
}

// NewExpectWithEnv creates a new process with user defined env variables for expect testing.
func NewExpectWithEnv(name string, args []string, env []string, serverProcessConfigName string) (ep *ExpectProcess, err error) {
	ep = &ExpectProcess{
		cfg: expectConfig{
			name: serverProcessConfigName,
			cmd:  name,
			args: args,
			env:  env,
		},
		readCloseCh: make(chan struct{}),
	}
	ep.cmd = commandFromConfig(ep.cfg)

	if ep.fpty, err = pty.Start(ep.cmd); err != nil {
		return nil, err
	}

	ep.wg.Add(2)
	go ep.read()
	go ep.waitSaveExitErr()
	return ep, nil
}

type expectConfig struct {
	name string
	cmd  string
	args []string
	env  []string
}

func commandFromConfig(config expectConfig) *exec.Cmd {
	cmd := exec.Command(config.cmd, config.args...)
	cmd.Env = config.env
	cmd.Stderr = cmd.Stdout
	cmd.Stdin = nil
	return cmd
}

func (ep *ExpectProcess) Pid() int {
	return ep.cmd.Process.Pid
}

func (ep *ExpectProcess) read() {
	defer func() {
		ep.wg.Done()
		close(ep.readCloseCh)
	}()
	defer func(fpty *os.File) {
		err := fpty.Close()
		if err != nil {
			// we deliberately only log the error here, closing the PTY should mostly be (expected) broken pipes
			fmt.Printf("error while closing fpty: %v", err)
		}
	}(ep.fpty)

	r := bufio.NewReader(ep.fpty)
	for {
		err := ep.tryReadNextLine(r)
		if err != nil {
			break
		}
	}
}

func (ep *ExpectProcess) tryReadNextLine(r *bufio.Reader) error {
	printDebugLines := os.Getenv("EXPECT_DEBUG") != ""
	l, err := r.ReadString('\n')

	ep.mu.Lock()
	defer ep.mu.Unlock()

	if l != "" {
		if printDebugLines {
			fmt.Printf("%s (%s) (%d): %s", ep.cmd.Path, ep.cfg.name, ep.cmd.Process.Pid, l)
		}
		ep.lines = append(ep.lines, l)
		ep.count++
	}

	// we're checking the error here at the bottom to ensure any leftover reads are still taken into account
	return err
}

func (ep *ExpectProcess) waitSaveExitErr() {
	defer ep.wg.Done()
	err := ep.waitProcess()

	ep.mu.Lock()
	defer ep.mu.Unlock()
	if err != nil {
		ep.exitErr = err
	}
}

// ExpectFunc returns the first line satisfying the function f.
func (ep *ExpectProcess) ExpectFunc(ctx context.Context, f func(string) bool) (string, error) {
	i := 0
	for {
		line, errsFound := func() (string, bool) {
			ep.mu.Lock()
			defer ep.mu.Unlock()

			// check if this expect has been already closed
			if ep.cmd == nil {
				return "", true
			}

			for i < len(ep.lines) {
				line := ep.lines[i]
				i++
				if f(line) {
					return line, false
				}
			}
			return "", ep.exitErr != nil
		}()

		if line != "" {
			return line, nil
		}

		if errsFound {
			break
		}

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("failed to find match string: %w", ctx.Err())
		case <-time.After(time.Millisecond * 10):
			// continue loop
		}
	}

	select {
	// NOTE: we wait readCloseCh for ep.read() to complete draining the log before acquring the lock.
	case <-ep.readCloseCh:
	case <-ctx.Done():
		return "", fmt.Errorf("failed to find match string: %w", ctx.Err())
	}

	ep.mu.Lock()
	defer ep.mu.Unlock()

	// retry it since we get all the log data
	for i < len(ep.lines) {
		line := ep.lines[i]
		i++
		if f(line) {
			return line, nil
		}
	}

	lastLinesIndex := len(ep.lines) - DEBUG_LINES_TAIL
	if lastLinesIndex < 0 {
		lastLinesIndex = 0
	}
	lastLines := strings.Join(ep.lines[lastLinesIndex:], "")
	return "", fmt.Errorf("match not found. "+
		" Set EXPECT_DEBUG for more info Errs: [%v], last lines:\n%s",
		ep.exitErr, lastLines)
}

// ExpectWithContext returns the first line containing the given string.
func (ep *ExpectProcess) ExpectWithContext(ctx context.Context, s ExpectedResponse) (string, error) {
	var (
		expr *regexp.Regexp
		err  error
	)
	if s.IsRegularExpr {
		expr, err = regexp.Compile(s.Value)
		if err != nil {
			return "", err
		}
	}
	return ep.ExpectFunc(ctx, func(txt string) bool {
		if expr != nil {
			return expr.MatchString(txt)
		}
		return strings.Contains(txt, s.Value)
	})
}

// Expect returns the first line containing the given string.
// Deprecated: please use ExpectWithContext instead.
func (ep *ExpectProcess) Expect(s string) (string, error) {
	return ep.ExpectWithContext(context.Background(), ExpectedResponse{Value: s})
}

// LineCount returns the number of recorded lines since
// the beginning of the process.
func (ep *ExpectProcess) LineCount() int {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.count
}

// ExitCode returns the exit code of this process.
// If the process is still running, it returns exit code 0 and ErrProcessRunning.
func (ep *ExpectProcess) ExitCode() (int, error) {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.cmd == nil {
		return ep.exitCode, nil
	}

	if ep.exitErr != nil {
		// If the child process panics or is killed, for instance, the
		// goFailpoint triggers the exit event, the ep.cmd isn't nil and
		// the exitCode will describe the case.
		if ep.exitCode != 0 {
			return ep.exitCode, nil
		}

		// If the wait4(2) in waitProcess returns error, the child
		// process might be reaped if the process handles the SIGCHILD
		// in other goroutine. It's unlikely in this repo. But we
		// should return the error for log even if the child process
		// is still running.
		return 0, ep.exitErr
	}

	return 0, ErrProcessRunning
}

// ExitError returns the exit error of this process (if any).
// If the process is still running, it returns ErrProcessRunning instead.
func (ep *ExpectProcess) ExitError() error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.cmd == nil {
		return ep.exitErr
	}

	return ErrProcessRunning
}

// Stop signals the process to terminate via SIGTERM
func (ep *ExpectProcess) Stop() error {
	err := ep.Signal(syscall.SIGTERM)
	if err != nil && errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

// Signal sends a signal to the expect process
func (ep *ExpectProcess) Signal(sig os.Signal) error {
	ep.mu.Lock()
	defer ep.mu.Unlock()

	if ep.cmd == nil {
		return errors.New("expect process already closed")
	}

	return ep.cmd.Process.Signal(sig)
}

func (ep *ExpectProcess) waitProcess() error {
	state, err := ep.cmd.Process.Wait()
	if err != nil {
		return err
	}

	ep.mu.Lock()
	defer ep.mu.Unlock()
	ep.exitCode = exitCode(state)

	if !state.Success() {
		return fmt.Errorf("unexpected exit code [%d] after running [%s]", ep.exitCode, ep.cmd.String())
	}

	return nil
}

// exitCode returns correct exit code for a process based on signaled or exited.
func exitCode(state *os.ProcessState) int {
	status := state.Sys().(syscall.WaitStatus)

	if status.Signaled() {
		return 128 + int(status.Signal())
	}
	return status.ExitStatus()
}

// Wait waits for the process to finish.
func (ep *ExpectProcess) Wait() {
	ep.wg.Wait()
}

// Close waits for the expect process to exit and return its error.
func (ep *ExpectProcess) Close() error {
	ep.wg.Wait()

	ep.mu.Lock()
	defer ep.mu.Unlock()

	// this signals to other funcs that the process has finished
	ep.cmd = nil
	return ep.exitErr
}

func (ep *ExpectProcess) Send(command string) error {
	_, err := io.WriteString(ep.fpty, command)
	return err
}

func (ep *ExpectProcess) Lines() []string {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	return ep.lines
}

// ReadLine returns line by line.
func (ep *ExpectProcess) ReadLine() string {
	ep.mu.Lock()
	defer ep.mu.Unlock()
	if ep.count > ep.cur {
		line := ep.lines[ep.cur]
		ep.cur++
		return line
	}
	return ""
}
