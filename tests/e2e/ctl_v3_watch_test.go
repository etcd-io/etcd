// Copyright 2018 The etcd Authors
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
)

type kvExec struct {
	key, val   string
	execOutput string
}

func setupWatchArgs(cx ctlCtx, args []string) []string {
	cmdArgs := append(cx.PrefixArgs(), "watch")
	if cx.interactive {
		cmdArgs = append(cmdArgs, "--interactive")
	} else {
		cmdArgs = append(cmdArgs, args...)
	}

	return cmdArgs
}

func ctlV3Watch(cx ctlCtx, args []string, kvs ...kvExec) error {
	cmdArgs := setupWatchArgs(cx, args)

	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}

	if cx.interactive {
		wl := strings.Join(append([]string{"watch"}, args...), " ") + "\r"
		if err = proc.Send(wl); err != nil {
			return err
		}
	}

	for _, elem := range kvs {
		if _, err = proc.Expect(elem.key); err != nil {
			return err
		}
		if _, err = proc.Expect(elem.val); err != nil {
			return err
		}
		if elem.execOutput != "" {
			if _, err = proc.Expect(elem.execOutput); err != nil {
				return err
			}
		}
	}
	return proc.Stop()
}

func ctlV3WatchFailPerm(cx ctlCtx, args []string) error {
	cmdArgs := setupWatchArgs(cx, args)

	proc, err := spawnCmd(cmdArgs)
	if err != nil {
		return err
	}

	if cx.interactive {
		wl := strings.Join(append([]string{"watch"}, args...), " ") + "\r"
		if err = proc.Send(wl); err != nil {
			return err
		}
	}

	// TODO(mitake): after printing accurate error message that includes
	// "permission denied", the above string argument of proc.Expect()
	// should be updated.
	_, err = proc.Expect("watch is canceled by the server")
	if err != nil {
		return err
	}
	return proc.Close()
}

func ctlV3Put(cx ctlCtx, key, value, leaseID string, flags ...string) error {
	skipValue := false
	skipLease := false
	for _, f := range flags {
		if f == "--ignore-value" {
			skipValue = true
		}
		if f == "--ignore-lease" {
			skipLease = true
		}
	}
	cmdArgs := append(cx.PrefixArgs(), "put", key)
	if !skipValue {
		cmdArgs = append(cmdArgs, value)
	}
	if leaseID != "" && !skipLease {
		cmdArgs = append(cmdArgs, "--lease", leaseID)
	}
	if len(flags) != 0 {
		cmdArgs = append(cmdArgs, flags...)
	}
	return spawnWithExpect(cmdArgs, "OK")
}

type kv struct {
	key, val string
}

func ctlV3Get(cx ctlCtx, args []string, kvs ...kv) error {
	cmdArgs := append(cx.PrefixArgs(), "get")
	cmdArgs = append(cmdArgs, args...)
	if !cx.quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	var lines []string
	for _, elem := range kvs {
		lines = append(lines, elem.key, elem.val)
	}
	return spawnWithExpects(cmdArgs, lines...)
}

// ctlV3GetWithErr runs "get" command expecting no output but error
func ctlV3GetWithErr(cx ctlCtx, args []string, errs []string) error {
	cmdArgs := append(cx.PrefixArgs(), "get")
	cmdArgs = append(cmdArgs, args...)
	if !cx.quorum {
		cmdArgs = append(cmdArgs, "--consistency", "s")
	}
	return spawnWithExpects(cmdArgs, errs...)
}

func ctlV3Del(cx ctlCtx, args []string, num int) error {
	cmdArgs := append(cx.PrefixArgs(), "del")
	cmdArgs = append(cmdArgs, args...)
	return spawnWithExpects(cmdArgs, fmt.Sprintf("%d", num))
}
