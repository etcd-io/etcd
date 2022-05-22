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
	"io"
	"strconv"
	"strings"

	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

type EtcdctlV3 struct {
	cfg       *EtcdProcessClusterConfig
	endpoints []string
}

func NewEtcdctl(cfg *EtcdProcessClusterConfig, endpoints []string) *EtcdctlV3 {
	return &EtcdctlV3{
		cfg:       cfg,
		endpoints: endpoints,
	}
}

func (ctl *EtcdctlV3) DowngradeEnable(version string) error {
	return SpawnWithExpect(ctl.cmdArgs("downgrade", "enable", version), "Downgrade enable success")
}

func (ctl *EtcdctlV3) Get(key string, o config.GetOptions) (*clientv3.GetResponse, error) {
	resp := clientv3.GetResponse{}
	var args []string
	if o.Timeout != 0 {
		args = append(args, fmt.Sprintf("--command-timeout=%s", o.Timeout))
	}
	if o.Serializable {
		args = append(args, "--consistency", "s")
	}
	args = append(args, "get", key, "-w", "json")
	if o.End != "" {
		args = append(args, o.End)
	}
	if o.Revision != 0 {
		args = append(args, fmt.Sprintf("--rev=%d", o.Revision))
	}
	if o.Prefix {
		args = append(args, "--prefix")
	}
	if o.Limit != 0 {
		args = append(args, fmt.Sprintf("--limit=%d", o.Limit))
	}
	if o.FromKey {
		args = append(args, "--from-key")
	}
	if o.CountOnly {
		args = append(args, "-w", "fields", "--count-only")
	} else {
		args = append(args, "-w", "json")
	}
	switch o.SortBy {
	case clientv3.SortByCreateRevision:
		args = append(args, "--sort-by=CREATE")
	case clientv3.SortByModRevision:
		args = append(args, "--sort-by=MODIFY")
	case clientv3.SortByValue:
		args = append(args, "--sort-by=VALUE")
	case clientv3.SortByVersion:
		args = append(args, "--sort-by=VERSION")
	case clientv3.SortByKey:
		// nothing
	default:
		return nil, fmt.Errorf("bad sort target %v", o.SortBy)
	}
	switch o.Order {
	case clientv3.SortAscend:
		args = append(args, "--order=ASCEND")
	case clientv3.SortDescend:
		args = append(args, "--order=DESCEND")
	case clientv3.SortNone:
		// nothing
	default:
		return nil, fmt.Errorf("bad sort order %v", o.Order)
	}
	if o.CountOnly {
		cmd, err := SpawnCmd(ctl.cmdArgs(args...), nil)
		if err != nil {
			return nil, err
		}
		_, err = cmd.Expect("Count")
		return &resp, err
	}
	err := ctl.spawnJsonCmd(&resp, args...)
	return &resp, err
}

func (ctl *EtcdctlV3) Put(key, value string, opts config.PutOptions) error {
	args := ctl.cmdArgs()
	args = append(args, "put", key, value)
	if opts.LeaseID != 0 {
		args = append(args, "--lease", strconv.FormatInt(int64(opts.LeaseID), 16))
	}
	return SpawnWithExpect(args, "OK")
}

func (ctl *EtcdctlV3) Delete(key string, o config.DeleteOptions) (*clientv3.DeleteResponse, error) {
	args := []string{"del", key}
	if o.End != "" {
		args = append(args, o.End)
	}
	if o.Prefix {
		args = append(args, "--prefix")
	}
	if o.FromKey {
		args = append(args, "--from-key")
	}
	var resp clientv3.DeleteResponse
	err := ctl.spawnJsonCmd(&resp, args...)
	return &resp, err
}

func (ctl *EtcdctlV3) Txn(compares, ifSucess, ifFail []string, o config.TxnOptions) (*clientv3.TxnResponse, error) {
	args := ctl.cmdArgs()
	args = append(args, "txn")
	if o.Interactive {
		args = append(args, "--interactive")
	}
	args = append(args, "-w", "json", "--hex=true")
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	_, err = cmd.Expect("compares:")
	if err != nil {
		return nil, err
	}
	for _, cmp := range compares {
		if err := cmd.Send(cmp + "\r"); err != nil {
			return nil, err
		}
	}
	if err := cmd.Send("\r"); err != nil {
		return nil, err
	}
	_, err = cmd.Expect("success requests (get, put, del):")
	if err != nil {
		return nil, err
	}
	for _, req := range ifSucess {
		if err = cmd.Send(req + "\r"); err != nil {
			return nil, err
		}
	}
	if err = cmd.Send("\r"); err != nil {
		return nil, err
	}

	_, err = cmd.Expect("failure requests (get, put, del):")
	if err != nil {
		return nil, err
	}
	for _, req := range ifFail {
		if err = cmd.Send(req + "\r"); err != nil {
			return nil, err
		}
	}
	if err = cmd.Send("\r"); err != nil {
		return nil, err
	}
	var line string
	line, err = cmd.Expect("header")
	if err != nil {
		return nil, err
	}
	var resp clientv3.TxnResponse
	AddTxnResponse(&resp, line)
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

// AddTxnResponse looks for ResponseOp json tags and adds the objects for json decoding
func AddTxnResponse(resp *clientv3.TxnResponse, jsonData string) {
	if resp == nil {
		return
	}
	if resp.Responses == nil {
		resp.Responses = []*etcdserverpb.ResponseOp{}
	}
	jd := json.NewDecoder(strings.NewReader(jsonData))
	for {
		t, e := jd.Token()
		if e == io.EOF {
			break
		}
		if t == "response_range" {
			resp.Responses = append(resp.Responses, &etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseRange{},
			})
		}
		if t == "response_put" {
			resp.Responses = append(resp.Responses, &etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponsePut{},
			})
		}
		if t == "response_delete_range" {
			resp.Responses = append(resp.Responses, &etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseDeleteRange{},
			})
		}
		if t == "response_txn" {
			resp.Responses = append(resp.Responses, &etcdserverpb.ResponseOp{
				Response: &etcdserverpb.ResponseOp_ResponseTxn{},
			})
		}
	}
}
func (ctl *EtcdctlV3) MemberList() (*clientv3.MemberListResponse, error) {
	var resp clientv3.MemberListResponse
	err := ctl.spawnJsonCmd(&resp, "member", "list")
	return &resp, err
}

func (ctl *EtcdctlV3) MemberAddAsLearner(name string, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
	var resp clientv3.MemberAddResponse
	err := ctl.spawnJsonCmd(&resp, "member", "add", name, "--learner", "--peer-urls", strings.Join(peerAddrs, ","))
	return &resp, err
}

func (ctl *EtcdctlV3) MemberRemove(id uint64) (*clientv3.MemberRemoveResponse, error) {
	var resp clientv3.MemberRemoveResponse
	err := ctl.spawnJsonCmd(&resp, "member", "remove", fmt.Sprintf("%x", id))
	return &resp, err
}

func (ctl *EtcdctlV3) cmdArgs(args ...string) []string {
	cmdArgs := []string{CtlBinPath + "3"}
	for k, v := range ctl.flags() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	return append(cmdArgs, args...)
}

func (ctl *EtcdctlV3) flags() map[string]string {
	fmap := make(map[string]string)
	if ctl.cfg.ClientTLS == ClientTLS {
		if ctl.cfg.IsClientAutoTLS {
			fmap["insecure-transport"] = "false"
			fmap["insecure-skip-tls-verify"] = "true"
		} else if ctl.cfg.IsClientCRL {
			fmap["cacert"] = CaPath
			fmap["cert"] = RevokedCertPath
			fmap["key"] = RevokedPrivateKeyPath
		} else {
			fmap["cacert"] = CaPath
			fmap["cert"] = CertPath
			fmap["key"] = PrivateKeyPath
		}
	}
	fmap["endpoints"] = strings.Join(ctl.endpoints, ",")
	return fmap
}

func (ctl *EtcdctlV3) Compact(rev int64, o config.CompactOption) (*clientv3.CompactResponse, error) {
	args := ctl.cmdArgs("compact", fmt.Sprint(rev))
	if o.Timeout != 0 {
		args = append(args, fmt.Sprintf("--command-timeout=%s", o.Timeout))
	}
	if o.Physical {
		args = append(args, "--physical")
	}

	return nil, SpawnWithExpect(args, fmt.Sprintf("compacted revision %v", rev))
}

func (ctl *EtcdctlV3) Status() ([]*clientv3.StatusResponse, error) {
	var epStatus []*struct {
		Endpoint string
		Status   *clientv3.StatusResponse
	}
	err := ctl.spawnJsonCmd(&epStatus, "endpoint", "status", "--endpoints", strings.Join(ctl.endpoints, ","))
	if err != nil {
		return nil, err
	}
	resp := make([]*clientv3.StatusResponse, len(epStatus))
	for _, e := range epStatus {
		resp = append(resp, e.Status)
	}
	return resp, err
}

func (ctl *EtcdctlV3) HashKV(rev int64) ([]*clientv3.HashKVResponse, error) {
	var epHashKVs []*struct {
		Endpoint string
		HashKV   *clientv3.HashKVResponse
	}
	err := ctl.spawnJsonCmd(&epHashKVs, "endpoint", "hashkv", "--endpoints", strings.Join(ctl.endpoints, ","), "--rev", fmt.Sprint(rev))
	if err != nil {
		return nil, err
	}
	resp := make([]*clientv3.HashKVResponse, len(epHashKVs))
	for _, e := range epHashKVs {
		resp = append(resp, e.HashKV)
	}
	return resp, err
}

func (ctl *EtcdctlV3) Health() error {
	args := ctl.cmdArgs()
	args = append(args, "endpoint", "health")
	lines := make([]string, len(ctl.endpoints))
	for i := range lines {
		lines[i] = "is healthy"
	}
	return SpawnWithExpects(args, map[string]string{}, lines...)
}

func (ctl *EtcdctlV3) Grant(ttl int64) (*clientv3.LeaseGrantResponse, error) {
	args := ctl.cmdArgs()
	args = append(args, "lease", "grant", strconv.FormatInt(ttl, 10), "-w", "json")
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.LeaseGrantResponse
	line, err := cmd.Expect("ID")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) TimeToLive(id clientv3.LeaseID, o config.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	args := ctl.cmdArgs()
	args = append(args, "lease", "timetolive", strconv.FormatInt(int64(id), 16), "-w", "json")
	if o.WithAttachedKeys {
		args = append(args, "--keys")
	}
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.LeaseTimeToLiveResponse
	line, err := cmd.Expect("id")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) Defragment(o config.DefragOption) error {
	args := append(ctl.cmdArgs(), "defrag")
	if o.Timeout != 0 {
		args = append(args, fmt.Sprintf("--command-timeout=%s", o.Timeout))
	}
	lines := make([]string, len(ctl.endpoints))
	for i := range lines {
		lines[i] = "Finished defragmenting etcd member"
	}
	_, err := SpawnWithExpectLines(args, map[string]string{}, lines...)
	return err
}

func (ctl *EtcdctlV3) LeaseList() (*clientv3.LeaseLeasesResponse, error) {
	args := ctl.cmdArgs("lease", "list", "-w", "json")
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.LeaseLeasesResponse
	line, err := cmd.Expect("id")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) LeaseKeepAliveOnce(id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
	args := ctl.cmdArgs("lease", "keep-alive", strconv.FormatInt(int64(id), 16), "--once", "-w", "json")
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.LeaseKeepAliveResponse
	line, err := cmd.Expect("ID")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) LeaseRevoke(id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	var resp clientv3.LeaseRevokeResponse
	err := ctl.spawnJsonCmd(&resp, "lease", "revoke", strconv.FormatInt(int64(id), 16))
	return &resp, err
}

func (ctl *EtcdctlV3) AlarmList() (*clientv3.AlarmResponse, error) {
	var resp clientv3.AlarmResponse
	err := ctl.spawnJsonCmd(&resp, "alarm", "list")
	return &resp, err
}

func (ctl *EtcdctlV3) AlarmDisarm(_ *clientv3.AlarmMember) (*clientv3.AlarmResponse, error) {
	args := ctl.cmdArgs()
	args = append(args, "alarm", "disarm", "-w", "json")
	ep, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.AlarmResponse
	line, err := ep.Expect("alarm")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) UserAdd(name, password string, opts config.UserAddOptions) (*clientv3.AuthUserAddResponse, error) {
	args := ctl.cmdArgs()
	args = append(args, "user", "add")
	if password == "" {
		args = append(args, name)
	} else {
		args = append(args, fmt.Sprintf("%s:%s", name, password))
	}

	if opts.NoPassword {
		args = append(args, "--no-password")
	}

	args = append(args, "--interactive=false", "-w", "json")

	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}

	// If no password is provided, and NoPassword isn't set, the CLI will always
	// wait for a password, send an enter in this case for an "empty" password.
	if !opts.NoPassword && password == "" {
		err := cmd.Send("\n")
		if err != nil {
			return nil, err
		}
	}

	var resp clientv3.AuthUserAddResponse
	line, err := cmd.Expect("header")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) UserList() (*clientv3.AuthUserListResponse, error) {
	var resp clientv3.AuthUserListResponse
	err := ctl.spawnJsonCmd(&resp, "user", "list")
	return &resp, err
}

func (ctl *EtcdctlV3) UserDelete(name string) (*clientv3.AuthUserDeleteResponse, error) {
	var resp clientv3.AuthUserDeleteResponse
	err := ctl.spawnJsonCmd(&resp, "user", "delete", name)
	return &resp, err
}

func (ctl *EtcdctlV3) UserChangePass(user, newPass string) error {
	args := ctl.cmdArgs()
	args = append(args, "user", "passwd", user, "--interactive=false")
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return err
	}

	err = cmd.Send(newPass + "\n")
	if err != nil {
		return err
	}

	_, err = cmd.Expect("Password updated")
	return err
}

func (ctl *EtcdctlV3) RoleAdd(name string) (*clientv3.AuthRoleAddResponse, error) {
	var resp clientv3.AuthRoleAddResponse
	err := ctl.spawnJsonCmd(&resp, "role", "add", name)
	return &resp, err
}

func (ctl *EtcdctlV3) RoleGrantPermission(name string, key, rangeEnd string, permType clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error) {
	permissionType := authpb.Permission_Type_name[int32(permType)]
	var resp clientv3.AuthRoleGrantPermissionResponse
	err := ctl.spawnJsonCmd(&resp, "role", "grant-permission", name, permissionType, key, rangeEnd)
	return &resp, err
}

func (ctl *EtcdctlV3) RoleGet(role string) (*clientv3.AuthRoleGetResponse, error) {
	var resp clientv3.AuthRoleGetResponse
	err := ctl.spawnJsonCmd(&resp, "role", "get", role)
	return &resp, err
}

func (ctl *EtcdctlV3) RoleList() (*clientv3.AuthRoleListResponse, error) {
	var resp clientv3.AuthRoleListResponse
	err := ctl.spawnJsonCmd(&resp, "role", "list")
	return &resp, err
}

func (ctl *EtcdctlV3) RoleRevokePermission(role string, key, rangeEnd string) (*clientv3.AuthRoleRevokePermissionResponse, error) {
	var resp clientv3.AuthRoleRevokePermissionResponse
	err := ctl.spawnJsonCmd(&resp, "role", "revoke-permission", role, key, rangeEnd)
	return &resp, err
}

func (ctl *EtcdctlV3) RoleDelete(role string) (*clientv3.AuthRoleDeleteResponse, error) {
	var resp clientv3.AuthRoleDeleteResponse
	err := ctl.spawnJsonCmd(&resp, "role", "delete", role)
	return &resp, err
}

func (ctl *EtcdctlV3) spawnJsonCmd(output interface{}, args ...string) error {
	args = append(args, "-w", "json")
	cmd, err := SpawnCmd(append(ctl.cmdArgs(), args...), nil)
	if err != nil {
		return err
	}
	line, err := cmd.Expect("header")
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(line), output)
}
