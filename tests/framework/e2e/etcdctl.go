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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

type EtcdctlV3 struct {
	cfg        ClientConfig
	endpoints  []string
	authConfig clientv3.AuthConfig
}

func NewEtcdctl(cfg ClientConfig, endpoints []string, opts ...config.ClientOption) (*EtcdctlV3, error) {
	ctl := &EtcdctlV3{
		cfg:       cfg,
		endpoints: endpoints,
	}

	for _, opt := range opts {
		opt(ctl)
	}

	if !ctl.authConfig.Empty() {
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   ctl.endpoints,
			DialTimeout: 5 * time.Second,
			DialOptions: []grpc.DialOption{grpc.WithBlock()},
			Username:    ctl.authConfig.Username,
			Password:    ctl.authConfig.Password,
		})
		if err != nil {
			return nil, err
		}
		client.Close()
	}

	return ctl, nil
}

func (ctl *EtcdctlV3) Get(ctx context.Context, key string, o config.GetOptions) (*clientv3.GetResponse, error) {
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
		defer cmd.Close()
		_, err = cmd.ExpectWithContext(ctx, expect.ExpectedResponse{Value: "Count"})
		return &resp, err
	}
	err := ctl.spawnJsonCmd(ctx, &resp, args...)
	return &resp, err
}

func (ctl *EtcdctlV3) Status(ctx context.Context) ([]*clientv3.StatusResponse, error) {
	var epStatus []*struct {
		Endpoint string
		Status   *clientv3.StatusResponse
	}
	err := ctl.spawnJsonCmd(ctx, &epStatus, "endpoint", "status")
	if err != nil {
		return nil, err
	}
	resp := make([]*clientv3.StatusResponse, len(epStatus))
	for i, e := range epStatus {
		resp[i] = e.Status
	}
	return resp, err
}

func (ctl *EtcdctlV3) cmdArgs(args ...string) []string {
	cmdArgs := []string{BinPath.Etcdctl}
	for k, v := range ctl.flags() {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", k, v))
	}
	return append(cmdArgs, args...)
}

func (ctl *EtcdctlV3) flags() map[string]string {
	fmap := make(map[string]string)
	if ctl.cfg.ConnectionType == ClientTLS {
		if ctl.cfg.AutoTLS {
			fmap["insecure-transport"] = "false"
			fmap["insecure-skip-tls-verify"] = "true"
		} else if ctl.cfg.RevokeCerts {
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
	if !ctl.authConfig.Empty() {
		fmap["user"] = ctl.authConfig.Username + ":" + ctl.authConfig.Password
	}
	return fmap
}

func (ctl *EtcdctlV3) spawnJsonCmd(ctx context.Context, output any, args ...string) error {
	args = append(args, "-w", "json")
	cmd, err := SpawnCmd(append(ctl.cmdArgs(), args...), nil)
	if err != nil {
		return err
	}
	defer cmd.Close()
	line, err := cmd.ExpectWithContext(ctx, expect.ExpectedResponse{Value: "header"})
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(line), output)
}
