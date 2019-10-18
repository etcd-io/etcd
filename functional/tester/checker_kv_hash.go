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

	"go.etcd.io/etcd/functional/rpcpb"

	"go.uber.org/zap"
)

const retries = 7

type kvHashChecker struct {
	ctype rpcpb.Checker
	clus  *Cluster
}

func newKVHashChecker(clus *Cluster) Checker {
	return &kvHashChecker{
		ctype: rpcpb.Checker_KV_HASH,
		clus:  clus,
	}
}

func (hc *kvHashChecker) checkRevAndHashes() (err error) {
	var (
		revs   map[string]int64
		hashes map[string]int64
	)
	// retries in case of transient failure or etcd cluster has not stablized yet.
	for i := 0; i < retries; i++ {
		revs, hashes, err = hc.clus.getRevisionHash()
		if err != nil {
			hc.clus.lg.Warn(
				"failed to get revision and hash",
				zap.Int("retries", i),
				zap.Error(err),
			)
		} else {
			sameRev := getSameValue(revs)
			sameHashes := getSameValue(hashes)
			if sameRev && sameHashes {
				return nil
			}
			hc.clus.lg.Warn(
				"retrying; etcd cluster is not stable",
				zap.Int("retries", i),
				zap.Bool("same-revisions", sameRev),
				zap.Bool("same-hashes", sameHashes),
				zap.String("revisions", fmt.Sprintf("%+v", revs)),
				zap.String("hashes", fmt.Sprintf("%+v", hashes)),
			)
		}
		time.Sleep(time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed revision and hash check (%v)", err)
	}

	return fmt.Errorf("etcd cluster is not stable: [revisions: %v] and [hashes: %v]", revs, hashes)
}

func (hc *kvHashChecker) Type() rpcpb.Checker {
	return hc.ctype
}

func (hc *kvHashChecker) EtcdClientEndpoints() []string {
	return hc.clus.EtcdClientEndpoints()
}

func (hc *kvHashChecker) Check() error {
	return hc.checkRevAndHashes()
}
