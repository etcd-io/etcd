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
	"go.etcd.io/etcd/tests/v3/integration"
)

type Etcdctl struct {
	connType  ClientConnType
	isAutoTLS bool
	endpoints []string
	v2        bool
}

func NewEtcdctl(endpoints []string, connType ClientConnType, isAutoTLS bool, v2 bool) *Etcdctl {
	return &Etcdctl{
		endpoints: endpoints,
		connType:  connType,
		isAutoTLS: isAutoTLS,
		v2:        v2,
	}
}

func (ctl *Etcdctl) Get(key string) (*clientv3.GetResponse, error) {
	var resp clientv3.GetResponse
	err := ctl.spawnJsonCmd(&resp, "get", key)
	return &resp, err
}

func (ctl *Etcdctl) Put(key, value string) error {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs()
	args = append(args, "put", key, value)
	return SpawnWithExpectWithEnv(args, ctl.env(), "OK")
}

func (ctl *Etcdctl) PutWithAuth(key, value, username, password string) error {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs()
	args = append(args, "--user", fmt.Sprintf("%s:%s", username, password), "put", key, value)
	return SpawnWithExpectWithEnv(args, ctl.env(), "OK")
}

func (ctl *Etcdctl) Set(key, value string) error {
	if !ctl.v2 {
		panic("Unsupported method for v3")
	}
	args := ctl.cmdArgs()
	args = append(args, "set", key, value)
	lines, err := RunUtilCompletion(args, ctl.env())
	if err != nil {
		return err
	}
	response := strings.ReplaceAll(strings.Join(lines, "\n"), "\r\n", "")
	if response != value {
		return fmt.Errorf("Got unexpected response %q, expected %q", response, value)
	}
	return nil
}

func (ctl *Etcdctl) AuthEnable() error {
	args := ctl.cmdArgs("auth", "enable")
	return SpawnWithExpectWithEnv(args, ctl.env(), "Authentication Enabled")
}

func (ctl *Etcdctl) UserGrantRole(user string, role string) (*clientv3.AuthUserGrantRoleResponse, error) {
	var resp clientv3.AuthUserGrantRoleResponse
	err := ctl.spawnJsonCmd(&resp, "user", "grant-role", user, role)
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
	err := ctl.spawnJsonCmd(&resp, args...)
	return &resp, err
}

func (ctl *Etcdctl) AlarmList() (*clientv3.AlarmResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.AlarmResponse
	err := ctl.spawnJsonCmd(&resp, "alarm", "list")
	return &resp, err
}

func (ctl *Etcdctl) MemberList() (*clientv3.MemberListResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.MemberListResponse
	err := ctl.spawnJsonCmd(&resp, "member", "list")
	return &resp, err
}

func (ctl *Etcdctl) MemberAdd(name string, peerURLs []string) (*clientv3.MemberAddResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.MemberAddResponse
	err := ctl.spawnJsonCmd(&resp, "member", "add", name, "--peer-urls", strings.Join(peerURLs, ","))
	return &resp, err
}

func (ctl *Etcdctl) MemberRemove(id uint64) (*clientv3.MemberRemoveResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	var resp clientv3.MemberRemoveResponse
	err := ctl.spawnJsonCmd(&resp, "member", "remove", fmt.Sprintf("%x", id))
	return &resp, err
}

func (ctl *Etcdctl) Compact(rev int64) (*clientv3.CompactResponse, error) {
	if ctl.v2 {
		panic("Unsupported method for v2")
	}
	args := ctl.cmdArgs("compact", fmt.Sprint(rev))
	return nil, SpawnWithExpectWithEnv(args, ctl.env(), fmt.Sprintf("compacted revision %v", rev))
}

func (ctl *Etcdctl) Status() ([]*clientv3.StatusResponse, error) {
	var epStatus []*struct {
		Endpoint string
		Status   *clientv3.StatusResponse
	}
	err := ctl.spawnJsonCmd(&epStatus, "endpoint", "status")
	if err != nil {
		return nil, err
	}
	resp := make([]*clientv3.StatusResponse, len(epStatus))
	for i, e := range epStatus {
		resp[i] = e.Status
	}
	return resp, err
}

func (ctl *Etcdctl) spawnJsonCmd(output interface{}, args ...string) error {
	args = append(args, "-w", "json")
	cmd, err := SpawnCmd(append(ctl.cmdArgs(), args...), ctl.env())
	if err != nil {
		return err
	}
	line, err := cmd.Expect("header")
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(line), output)
}

func (ctl *Etcdctl) cmdArgs(args ...string) []string {
	cmdArgs := []string{CtlBinPath}
	for k, v := range ctl.flags() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	return append(cmdArgs, args...)
}

func (ctl *Etcdctl) flags() map[string]string {
	fmap := make(map[string]string)
	if ctl.v2 {
		fmap["no-sync"] = "true"
		if ctl.connType == ClientTLS {
			fmap["ca-file"] = integration.TestTLSInfo.TrustedCAFile
			fmap["cert-file"] = integration.TestTLSInfo.CertFile
			fmap["key-file"] = integration.TestTLSInfo.KeyFile
		}
	} else {
		if ctl.connType == ClientTLS {
			if ctl.isAutoTLS {
				fmap["insecure-transport"] = "false"
				fmap["insecure-skip-tls-verify"] = "true"
			} else {
				fmap["cacert"] = integration.TestTLSInfo.TrustedCAFile
				fmap["cert"] = integration.TestTLSInfo.CertFile
				fmap["key"] = integration.TestTLSInfo.KeyFile
			}
		}
	}
	fmap["endpoints"] = strings.Join(ctl.endpoints, ",")
	return fmap
}

func (ctl *Etcdctl) env() map[string]string {
	env := make(map[string]string)
	if ctl.v2 {
		env["ETCDCTL_API"] = "2"
	} else {
		env["ETCDCTL_API"] = "3"
	}
	return env
}
