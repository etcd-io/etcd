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

package e2e

import (
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

type txnRequests struct {
	compare  []string
	ifSucess []string
	ifFail   []string
	results  []string
}

func ctlV3Txn(cx ctlCtx, rqs txnRequests) error {
	// TODO: support non-interactive mode
	cmdArgs := append(cx.PrefixArgs(), "txn")
	if cx.interactive {
		cmdArgs = append(cmdArgs, "--interactive")
	}
	proc, err := e2e.SpawnCmd(cmdArgs, cx.envMap)
	if err != nil {
		return err
	}
	_, err = proc.Expect("compares:")
	if err != nil {
		return err
	}
	for _, req := range rqs.compare {
		if err = proc.Send(req + "\r"); err != nil {
			return err
		}
	}
	if err = proc.Send("\r"); err != nil {
		return err
	}

	_, err = proc.Expect("success requests (get, put, del):")
	if err != nil {
		return err
	}
	for _, req := range rqs.ifSucess {
		if err = proc.Send(req + "\r"); err != nil {
			return err
		}
	}
	if err = proc.Send("\r"); err != nil {
		return err
	}

	_, err = proc.Expect("failure requests (get, put, del):")
	if err != nil {
		return err
	}
	for _, req := range rqs.ifFail {
		if err = proc.Send(req + "\r"); err != nil {
			return err
		}
	}
	if err = proc.Send("\r"); err != nil {
		return err
	}

	for _, line := range rqs.results {
		_, err = proc.Expect(line)
		if err != nil {
			return err
		}
	}
	return proc.Close()
}
