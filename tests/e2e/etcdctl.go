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
	"encoding/json"
	"fmt"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type EtcdctlV3 struct {
	endpoints []string
}

func NewEtcdctl(endpoints []string) *EtcdctlV3 {
	return &EtcdctlV3{
		endpoints: endpoints,
	}
}

func (ctl *EtcdctlV3) Put(key, value string) error {
	args := ctl.cmdArgs()
	args = append(args, "put", key, value)
	return spawnWithExpect(args, "OK")
}

func (ctl *EtcdctlV3) AlarmList() (*clientv3.AlarmResponse, error) {
	var resp clientv3.AlarmResponse
	err := ctl.spawnJsonCmd(&resp, "alarm", "list")
	return &resp, err
}

func (ctl *EtcdctlV3) MemberList() (*clientv3.MemberListResponse, error) {
	var resp clientv3.MemberListResponse
	err := ctl.spawnJsonCmd(&resp, "member", "list")
	return &resp, err
}

func (ctl *EtcdctlV3) Compact(rev int64) (*clientv3.CompactResponse, error) {
	args := ctl.cmdArgs("compact", fmt.Sprint(rev))
	return nil, spawnWithExpect(args, fmt.Sprintf("compacted revision %v", rev))
}

func (ctl *EtcdctlV3) spawnJsonCmd(output interface{}, args ...string) error {
	args = append(args, "-w", "json")
	cmd, err := spawnCmd(append(ctl.cmdArgs(), args...), nil)
	if err != nil {
		return err
	}
	line, err := cmd.Expect("header")
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(line), output)
}

func (ctl *EtcdctlV3) cmdArgs(args ...string) []string {
	cmdArgs := []string{ctlBinPath + "3"}
	for k, v := range ctl.flags() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	return append(cmdArgs, args...)
}

func (ctl *EtcdctlV3) flags() map[string]string {
	fmap := make(map[string]string)
	fmap["endpoints"] = strings.Join(ctl.endpoints, ",")
	return fmap
}
