// Copyright 2017 The etcd Authors
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

package e2e

import (
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/pkg/expect"
)

func waitReadyExpectProc(exproc *expect.ExpectProcess, readyStrs []string) error {
	c := 0
	matchSet := func(l string) bool {
		for _, s := range readyStrs {
			if strings.Contains(l, s) {
				c++
				break
			}
		}
		return c == len(readyStrs)
	}
	_, err := exproc.ExpectFunc(matchSet)
	return err
}

func spawnWithExpect(args []string, expected string) error {
	return spawnWithExpects(args, []string{expected}...)
}

func spawnWithExpects(args []string, xs ...string) error {
	proc, err := spawnCmd(args)
	if err != nil {
		return err
	}
	// process until either stdout or stderr contains
	// the expected string
	var (
		lines    []string
		lineFunc = func(txt string) bool { return true }
	)
	for _, txt := range xs {
		for {
			l, lerr := proc.ExpectFunc(lineFunc)
			if lerr != nil {
				proc.Close()
				return fmt.Errorf("%v (expected %q, got %q)", lerr, txt, lines)
			}
			lines = append(lines, l)
			if strings.Contains(l, txt) {
				break
			}
		}
	}
	perr := proc.Close()
	if len(xs) == 0 && proc.LineCount() != noOutputLineCount { // expect no output
		return fmt.Errorf("unexpected output (got lines %q, line count %d)", lines, proc.LineCount())
	}
	return perr
}

func closeWithTimeout(p *expect.ExpectProcess, d time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- p.Close() }()
	select {
	case err := <-errc:
		return err
	case <-time.After(d):
		p.Stop()
		// retry close after stopping to collect SIGQUIT data, if any
		closeWithTimeout(p, time.Second)
	}
	return fmt.Errorf("took longer than %v to Close process %+v", d, p)
}

func toTLS(s string) string {
	return strings.Replace(s, "http://", "https://", 1)
}
