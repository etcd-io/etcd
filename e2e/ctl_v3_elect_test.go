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
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coreos/etcd/pkg/expect"
)

func TestCtlV3Elect(t *testing.T) { testCtl(t, electTest) }

var tFunc = func(txt string) bool { return true }

func electTest(cx ctlCtx) {
	defer close(cx.errc)

	var (
		eName   = "a"
		pName   = "b"
		timeout = 2 * time.Second
	)
	// propose through election
	node, nodeEctn, err := ctlV3ElectSpawn(cx, eName, pName, false, timeout)
	if err != nil {
		cx.t.Fatal(err)
	}

	// proposal gets elected
	listener, es2, err := ctlV3ElectSpawn(cx, eName, pName, true, timeout)
	defer listener.Stop()
	if err != nil {
		node.Stop()
		cx.t.Fatal(err)
	}
	if nodeEctn != es2 {
		node.Stop()
		cx.t.Fatalf("expected %q, got %q", nodeEctn, es2)
	}

	// same election should be blocked
	blocked, _, err := ctlV3ElectSpawn(cx, eName, "x", false, timeout)
	if err != nil {
		if !strings.Contains(err.Error(), "timed out") {
			cx.t.Fatal(err)
		}
	}
	bid, err := syscall.Getpgid(blocked.Pid())
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = syscall.Kill(-bid, syscall.SIGINT); err != nil {
		cx.t.Fatal(err)
	}
	if err = blocked.Close(); err != nil {
		if !strings.Contains(err.Error(), "interrupt") {
			cx.t.Fatal(err)
		}
	}

	// kill the node holding the election(leadership)
	nid, err := syscall.Getpgid(node.Pid())
	if err != nil {
		cx.t.Fatal(err)
	}
	if err = syscall.Kill(-nid, syscall.SIGINT); err != nil {
		cx.t.Fatal(err)
	}
	if err = blocked.Close(); err != nil {
		if !strings.Contains(err.Error(), "interrupt") {
			cx.t.Fatal(err)
		}
	}

	// ensure the election lock gets released
	nd, ndEctn, err := ctlV3ElectSpawn(cx, eName, "y", false, timeout)
	defer nd.Stop()
	if err != nil {
		cx.t.Fatal(err)
	}
	if nodeEctn == ndEctn {
		cx.t.Fatalf("expected different election name, got %q == %q", nodeEctn, ndEctn)
	}

	// ensure new election, proposal get watched
	listenerEtcn, err := listener.ExpectFunc(tFunc)
	if err != nil {
		cx.t.Fatal(err)
	}
	listenerEtcn = strings.Replace(listenerEtcn, "\r\n", "", -1)
	if ndEctn != listenerEtcn {
		cx.t.Fatalf("expected proposal %q, got %q", ndEctn, listenerEtcn)
	}
}

// ctlV3ElectSpawn spawns elect command without stopping(or closing) it.
// Make sure to stop it to prevent leaky goroutine.
func ctlV3ElectSpawn(cx ctlCtx, name, proposal string, listen bool, timeout time.Duration) (*expect.ExpectProcess, string, error) {
	cmdArgs := append(ctlV3PrefixArgs(cx.epc, cx.dialTimeout), "elect", name)
	if !listen {
		cmdArgs = append(cmdArgs, proposal)
	} else { // observation mode
		cmdArgs = append(cmdArgs, "--listen")
	}

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
		return proc, rs, fmt.Errorf("timed out %v", timeout)
	case <-donec:
	}

	if !strings.HasPrefix(rs, name) {
		return proc, rs, fmt.Errorf("rs expected %q, got %q", name, rs)
	}

	pName, err := proc.ExpectFunc(tFunc)
	if err != nil {
		return proc, rs, err
	}
	pName = strings.TrimSpace(strings.Replace(pName, "\r\n", "", -1))

	if pName != proposal {
		return proc, rs, fmt.Errorf("pName expect %q, got %q", proposal, pName)
	}
	return proc, rs, nil
}
