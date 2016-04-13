// Copyright 2016 CoreOS, Inc.
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
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/expect"
)

func TestCtlV3Lock(t *testing.T) { testCtl(t, lockTest) }

func lockTest(cx ctlCtx) {
	defer close(cx.errc)

	var (
		name    = "a"
		timeout = 2 * time.Second
	)
	node1, es1, err := ctlV3Lock(cx, name, timeout)
	if err != nil {
		cx.t.Fatal(err)
	}

	// deadlock (should be blocked)
	blocked, _, err := ctlV3Lock(cx, name, timeout)
	if err != nil {
		if !strings.Contains(err.Error(), "timed out") {
			cx.t.Fatal(err)
		}
	}
	// TODO: exec.Cmd.Process.Pid is actually ppid in this case. Does not work.
	if err := blocked.Signal(os.Interrupt); err != nil {
		cx.t.Fatal(err)
	}
	if err = blocked.Close(); err != nil {
		cx.t.Fatal(err)
	}
	//
	// bid, err := syscall.Getpgid(blocked.Pid())
	// if err != nil {
	// 	cx.t.Fatal(err)
	// }
	// if err = syscall.Kill(-bid, syscall.SIGINT); err != nil {
	// 	cx.t.Fatal(err)
	// }
	// if err = blocked.Close(); err != nil {
	// 	if !strings.Contains(err.Error(), "interrupt") {
	// 		cx.t.Fatal(err)
	// 	}
	// }

	// kill the node1 holding the lock
	pgid, err := syscall.Getpgid(node1.Pid())
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = syscall.Kill(-pgid, syscall.SIGINT); err != nil {
		cx.t.Fatal(err)
	}
	if err = node1.Close(); err != nil {
		if !strings.Contains(err.Error(), "interrupt") {
			cx.t.Fatal(err)
		}
	}

	// ensure the lock gets released
	node2, es2, err := ctlV3Lock(cx, name, timeout)
	defer node2.Stop()
	if err != nil {
		cx.t.Fatal(err)
	}
	if es1 == es2 {
		cx.t.Fatalf("expected different election name, got %q == %q", es1, es2)
	}
}

// ctlV3Lock spawns elect command without stopping(or closing) it.
// Make sure to stop it to prevent leaky goroutine.
func ctlV3Lock(cx ctlCtx, name string, timeout time.Duration) (*expect.ExpectProcess, string, error) {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "lock", name)
	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return proc, "", err
	}

	var (
		rs    string
		donec = make(chan struct{})
	)
	go func() {
		rs, _ = proc.ExpectFunc(tFunc)
		rs = strings.TrimSpace(strings.Replace(rs, "\r\n", "", -1))
		close(donec)
	}()
	select {
	case <-time.After(timeout):
		return proc, "", fmt.Errorf("timed out %v", timeout)
	case <-donec:
	}

	if !strings.HasPrefix(rs, name) {
		err = fmt.Errorf("name expected %q, got %q", name, rs)
	}
	return proc, rs, err
}
