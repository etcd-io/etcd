// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func CheckEndOfTestHashKV(ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	c, err := clientv3.New(clientv3.Config{
		Endpoints:            clus.EndpointsGRPC(),
		Logger:               zap.NewNop(),
		DialKeepAliveTime:    10 * time.Second,
		DialKeepAliveTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		return err
	}
	defer c.Close()

	hashKV, err := c.HashKV(ctx, clus.EndpointsGRPC()[0], 0)
	if err != nil {
		return err
	}
	rev := hashKV.Header.Revision

	hashKVs := make([]*clientv3.HashKVResponse, 0)
	hashKVs = append(hashKVs, hashKV)

	for _, member := range clus.Procs {
		hashKV, err := c.HashKV(ctx, member.EndpointsGRPC()[0], rev)
		if err != nil {
			return err
		}
		if hashKV.Header.Revision != rev {
			return fmt.Errorf("latest revision at the end of the test between nodes should be the same. Want %v, get %v", rev, hashKV.Header.Revision)
		}
		hashKVs = append(hashKVs, hashKV)
	}

	for i := 1; i < len(hashKVs); i++ {
		if hashKVs[i-1].CompactRevision != hashKVs[i].CompactRevision {
			return fmt.Errorf("compactRevision mismatch, node %v has %+v, node %v has %+v", hashKVs[i-1].Header.MemberId, hashKVs[i-1], hashKVs[i].Header.MemberId, hashKVs[i])
		}
		if hashKVs[i-1].Hash != hashKVs[i].Hash {
			return fmt.Errorf("hash mismatch, node %v has %+v, node %v has %+v", hashKVs[i-1].Header.MemberId, hashKVs[i-1], hashKVs[i].Header.MemberId, hashKVs[i])
		}
	}
	return nil
}
