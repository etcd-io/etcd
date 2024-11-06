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
	"time"

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

func (ctl *Etcdctl) PutWithAuth(key, value, username, password string) error {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs()
	args = append(args, "--user", fmt.Sprintf("%s:%s", username, password), "put", key, value)
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

func (ctl *Etcdctl) AuthEnable() error {
	args := ctl.cmdArgs("auth", "enable")
	return spawnWithExpect(args, "Authentication Enabled")
}

func (ctl *Etcdctl) UserGrantRole(user string, role string) (*clientv3.AuthUserGrantRoleResponse, error) {
	var resp clientv3.AuthUserGrantRoleResponse
	err := ctl.spawnJsonCmd(&resp, "", "user", "grant-role", user, role)
	return &resp, err
}

func (ctl *Etcdctl) UserAdd(name, password string) (*clientv3.AuthUserAddResponse, error) {
	args := []string{"user", "add"}
	if password == "" {
		args = append(args, name)
		args = append(args, "--no-password")
	} else {
		args = append(args, fmt.Sprintf("%s:%s", name, password))
	}
	args = append(args, "--interactive=false")

	var resp clientv3.AuthUserAddResponse
	err := ctl.spawnJsonCmd(&resp, "", args...)
	return &resp, err
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

func (ctl *Etcdctl) MemberAdd(name string, peerURLs []string) (*clientv3.MemberAddResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.MemberAddResponse
	err := ctl.spawnJsonCmd(&resp, "", "member", "add", name, "--peer-urls", strings.Join(peerURLs, ","))
	return &resp, err
}

func (ctl *Etcdctl) MemberRemove(id uint64) (*clientv3.MemberRemoveResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.MemberRemoveResponse
	err := ctl.spawnJsonCmd(&resp, "", "member", "remove", fmt.Sprintf("%x", id))
	return &resp, err
}

func (ctl *Etcdctl) Compact(rev int64) (*clientv3.CompactResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs("compact", fmt.Sprint(rev))
	return nil, spawnWithExpect(args, fmt.Sprintf("compacted revision %v", rev))
}

func (ctl *Etcdctl) Defragment(timeout time.Duration) error {
	args := append(ctl.cmdArgs(), "defrag")
	if timeout != 0 {
		args = append(args, fmt.Sprintf("--command-timeout=%s", timeout))
	}

	return spawnWithExpect(args, "Finished defragmenting etcd member")
}

func (ctl *Etcdctl) Status() ([]*clientv3.StatusResponse, error) {
	var epStatus []*struct {
		Endpoint string
		Status   *clientv3.StatusResponse
	}
	err := ctl.spawnJsonCmd(&epStatus, "", "endpoint", "status")
	if err != nil {
		return nil, err
	}
	resp := make([]*clientv3.StatusResponse, len(epStatus))
	for i, e := range epStatus {
		resp[i] = e.Status
	}
	return resp, err
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
