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

	"go.etcd.io/etcd/clientv3"
)

type Etcdctl struct {
	connType  clientConnType
	isAutoTLS bool
	endpoints []string
	v2        bool
}

func NewEtcdctl(endpoints []string, connType clientConnType, isAutoTLS bool, v2 bool) *Etcdctl {
	return &Etcdctl{
		endpoints: endpoints,
		connType:  connType,
		isAutoTLS: isAutoTLS,
		v2:        v2,
	}
}

func (ctl *Etcdctl) Get(key string) (*clientv3.GetResponse, error) {
	var resp clientv3.GetResponse
	err := ctl.spawnJsonCmd(&resp, "", "get", key)
	return &resp, err
}

func (ctl *Etcdctl) Put(key, value string) error {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs()
	args = append(args, "put", key, value)
	return spawnWithExpect(args, "OK")
}

func (ctl *Etcdctl) Set(key, value string) error {
	if !ctl.v2 {
		panic("Unsupported method for v3")
	}
	args := ctl.cmdArgs()
	args = append(args, "set", key, value)
	return spawnWithExpect(args, value)
}

func (ctl *Etcdctl) AlarmList() (*clientv3.AlarmResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.AlarmResponse
	err := ctl.spawnJsonCmd(&resp, "{", "alarm", "list")
	return &resp, err
}

func (ctl *Etcdctl) MemberList() (*clientv3.MemberListResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.MemberListResponse
	err := ctl.spawnJsonCmd(&resp, "", "member", "list")
	return &resp, err
}

func (ctl *Etcdctl) Compact(rev int64) (*clientv3.CompactResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs("compact", fmt.Sprint(rev))
	return nil, spawnWithExpect(args, fmt.Sprintf("compacted revision %v", rev))
}

func (ctl *Etcdctl) spawnJsonCmd(output interface{}, expectedOutput string, args ...string) error {
	args = append(args, "-w", "json")
	cmd, err := spawnCmd(append(ctl.cmdArgs(), args...))
	if err != nil {
		return err
	}
	if expectedOutput == "" {
		expectedOutput = "header"
	}
	line, err := cmd.Expect(expectedOutput)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(line), output)
}

func (ctl *Etcdctl) cmdArgs(args ...string) []string {
	var cmdArgs []string
	binPath := ctlBinPath
	if ctl.v2 {
		cmdArgs = []string{binPath + "2"}
	} else {
		cmdArgs = []string{binPath + "3"}
	}
	for k, v := range ctl.flags() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	return append(cmdArgs, args...)
}

func (ctl *Etcdctl) flags() map[string]string {
	fmap := make(map[string]string)
	if ctl.v2 {
		fmap["no-sync"] = "true"
		if ctl.connType == clientTLS {
			fmap["ca-file"] = testTLSInfo.TrustedCAFile
			fmap["cert-file"] = testTLSInfo.CertFile
			fmap["key-file"] = testTLSInfo.KeyFile
		}
	} else {
		if ctl.connType == clientTLS {
			if ctl.isAutoTLS {
				fmap["insecure-transport"] = "false"
				fmap["insecure-skip-tls-verify"] = "true"
			} else {
				fmap["cacert"] = testTLSInfo.TrustedCAFile
				fmap["cert"] = testTLSInfo.CertFile
				fmap["key"] = testTLSInfo.KeyFile
			}
		}
	}
	fmap["endpoints"] = strings.Join(ctl.endpoints, ",")
	return fmap
}
