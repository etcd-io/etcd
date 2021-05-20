// Copyright 2018 The etcd Authors
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

package tester

import (
	"fmt"
	"time"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type shortTTLLeaseExpireChecker struct {
	ctype rpcpb.Checker
	lg    *zap.Logger
	m     *rpcpb.Member
	ls    *leaseStresser
	cli   *clientv3.Client
}

func newShortTTLLeaseExpireChecker(ls *leaseStresser) Checker {
	return &shortTTLLeaseExpireChecker{
		ctype: rpcpb.Checker_SHORT_TTL_LEASE_EXPIRE,
		lg:    ls.lg,
		m:     ls.m,
		ls:    ls,
	}
}

func (lc *shortTTLLeaseExpireChecker) Type() rpcpb.Checker {
	return lc.ctype
}

func (lc *shortTTLLeaseExpireChecker) EtcdClientEndpoints() []string {
	return []string{lc.m.EtcdClientEndpoint}
}

func (lc *shortTTLLeaseExpireChecker) Check() error {
	if lc.ls == nil {
		return nil
	}
	if lc.ls != nil && lc.ls.alivedLeasesWithShortTTL == nil {
		return nil
	}

	cli, err := lc.m.CreateEtcdClient(grpc.WithBackoffMaxDelay(time.Second))
	if err != nil {
		return fmt.Errorf("%v (%q)", err, lc.m.EtcdClientEndpoint)
	}
	defer func() {
		if cli != nil {
			cli.Close()
		}
	}()
	lc.cli = cli
	if err := check(lc.lg, lc.cli, false, lc.ls.alivedLeasesWithShortTTL.leases); err != nil {
		lc.lg.Error("failed to check alivedLeasesWithShortTTL", zap.Error(err))
		return err
	}
	lc.lg.Info("check alivedLeasesWithShortTTL succ", zap.Int("num", len(lc.ls.alivedLeasesWithShortTTL.leases)))
	return nil
}
