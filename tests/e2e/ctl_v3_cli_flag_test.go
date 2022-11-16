// Copyright 2022 The etcd Authors
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
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestEtcdctlDurationFlags(t *testing.T) {
	e2e.SkipInShortMode(t)

	timeDur := 60 * time.Second
	timeDurStr := timeDur.String()
	inputFlags := []string{
		"dial-timeout",
		"command-timeout",
		"keepalive-time",
		"keepalive-timeout",
	}

	etcdCmd := []string{e2e.BinPath.Etcdctl, "--debug", "version"}
	outExpectedStrs := []string{}
	for _, inFlag := range inputFlags {
		flagWithHyphens := fmt.Sprintf("--%s", inFlag)
		etcdCmd = append(etcdCmd, flagWithHyphens, timeDurStr)
		outExpectedStrs = append(outExpectedStrs, fmt.Sprintf(`"%s":"%s"`, inFlag, timeDur))
	}

	proc, err := e2e.SpawnCmd(etcdCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = e2e.WaitReadyExpectProc(context.TODO(), proc, outExpectedStrs); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestEtcdctlDurationFlags_IntegerInputValues(t *testing.T) {
	e2e.SkipInShortMode(t)

	timeDurIntFormat := 88
	timeDurIntFormatStr := strconv.Itoa(timeDurIntFormat) // This is the actual check: input should be an (duration) integer format string
	outTimeDur := time.Duration(timeDurIntFormat) * time.Second
	inputFlags := []string{
		"dial-timeout",
		"command-timeout",
		"keepalive-time",
		"keepalive-timeout",
	}

	etcdCmd := []string{e2e.BinPath.Etcdctl, "--debug", "version"}
	outExpectedStrs := []string{}
	for _, inFlag := range inputFlags {
		flagWithHyphens := fmt.Sprintf("--%s", inFlag)
		etcdCmd = append(etcdCmd, flagWithHyphens, timeDurIntFormatStr)
		outExpectedStrs = append(outExpectedStrs, fmt.Sprintf(`"%s":"%s"`, inFlag, outTimeDur))
	}

	proc, err := e2e.SpawnCmd(etcdCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = e2e.WaitReadyExpectProc(context.TODO(), proc, outExpectedStrs); err != nil {
		t.Fatal(err)
	}
	if err = proc.Stop(); err != nil {
		t.Fatal(err)
	}
}
