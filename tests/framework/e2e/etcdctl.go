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
	args := ctl.cmdArgs()
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
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.GetResponse
	if o.CountOnly {
		_, err := cmd.Expect("Count")
		return &resp, err
	}
	line, err := cmd.Expect("header")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) Put(key, value string) error {
	return SpawnWithExpect(ctl.cmdArgs("put", key, value), "OK")
}

func (ctl *EtcdctlV3) Delete(key string, o config.DeleteOptions) (*clientv3.DeleteResponse, error) {
	args := ctl.cmdArgs()
	args = append(args, "del", key, "-w", "json")
	if o.End != "" {
		args = append(args, o.End)
	}
	if o.Prefix {
		args = append(args, "--prefix")
	}
	if o.FromKey {
		args = append(args, "--from-key")
	}
	cmd, err := SpawnCmd(args, nil)
	if err != nil {
		return nil, err
	}
	var resp clientv3.DeleteResponse
	line, err := cmd.Expect("header")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(line), &resp)
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
	args := ctl.cmdArgs()
	args = append(args, "compact", fmt.Sprint(rev))

	if o.Timeout != 0 {
		args = append(args, fmt.Sprintf("--command-timeout=%s", o.Timeout))
	}
	if o.Physical {
		args = append(args, "--physical")
	}

	return nil, SpawnWithExpect(args, fmt.Sprintf("compacted revision %v", rev))
}
