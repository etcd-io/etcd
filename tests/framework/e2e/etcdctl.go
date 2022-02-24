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
	"go.etcd.io/etcd/tests/v3/framework/testutils"
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

func (ctl *EtcdctlV3) Get(key string, o testutils.GetOptions) (*clientv3.GetResponse, error) {
	args := ctl.cmdArgs()
	if o.Serializable {
		args = append(args, "--consistency", "s")
	}
	cmd, err := SpawnCmd(append(args, "get", key, "-w", "json"), nil)
	if err != nil {
		return nil, err
	}
	line, err := cmd.Expect("kvs")
	if err != nil {
		return nil, err
	}
	var resp clientv3.GetResponse
	err = json.Unmarshal([]byte(line), &resp)
	return &resp, err
}

func (ctl *EtcdctlV3) Put(key, value string) error {
	return SpawnWithExpect(ctl.cmdArgs("put", key, value), "OK")
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
