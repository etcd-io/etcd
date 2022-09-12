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
	"context"
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
	cfg expectConfig

	cmd  *exec.Cmd
	fpty *os.File
	wg   sync.WaitGroup

	mu    sync.Mutex // protects lines and err
	lines []string
	count int // increment whenever new line gets added
	cur   int // current read position
	err   error
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
	}
	ep.cmd = commandFromConfig(ep.cfg)

	if ep.fpty, err = pty.Start(ep.cmd); err != nil {
		return nil, err
	}

	ep.wg.Add(1)
	go ep.read()
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
	defer ep.wg.Done()
	printDebugLines := os.Getenv("EXPECT_DEBUG") != ""
	r := bufio.NewReader(ep.fpty)
	for {
		l, err := r.ReadString('\n')
		ep.mu.Lock()
		if l != "" {
			if printDebugLines {
				fmt.Printf("%s (%s) (%d): %s", ep.cmd.Path, ep.cfg.name, ep.cmd.Process.Pid, l)
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
func (ep *ExpectProcess) ExpectFunc(ctx context.Context, f func(string) bool) (string, error) {
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

		select {
		case <-ctx.Done():
			return "", fmt.Errorf("failed to find match string: %w", ctx.Err())
		case <-time.After(time.Millisecond * 10):
			// continue loop
		}
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

// ExpectWithContext returns the first line containing the given string.
func (ep *ExpectProcess) ExpectWithContext(ctx context.Context, s string) (string, error) {
	return ep.ExpectFunc(ctx, func(txt string) bool { return strings.Contains(txt, s) })
}

// Expect returns the first line containing the given string.
// Deprecated: please use ExpectWithContext instead.
func (ep *ExpectProcess) Expect(s string) (string, error) {
	return ep.ExpectWithContext(context.Background(), s)
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

// Close waits for the expect process to exit.
// Close currently does not return error if process exited with !=0 status.
// TODO: Close should expose underlying process failure by default.
func (ep *ExpectProcess) Close() error { return ep.close(false) }

func (ep *ExpectProcess) close(kill bool) error {
	if ep.cmd == nil {
		return ep.err
	}
	if kill {
		ep.Signal(syscall.SIGTERM)
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
