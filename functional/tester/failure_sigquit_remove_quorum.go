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
	"context"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/functional/rpcpb"

	"go.uber.org/zap"
)

type fetchSnapshotAndFailureQuorum struct {
	desc        string
	failureCase rpcpb.FailureCase
	injected    map[int]struct{}
	snapshotted int
}

func (f *fetchSnapshotAndFailureQuorum) Inject(clus *Cluster) error {
	//  1. Assume node C is the current leader with most up-to-date data.
	lead, err := clus.GetLeader()
	if err != nil {
		return err
	}
	f.snapshotted = lead

	//  2. Download snapshot from node C, before destroying node A and B.
	clus.lg.Info(
		"install snapshot on leader node START",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
		zap.Error(err),
	)
	var resp *rpcpb.Response
	if resp == nil || err != nil {
		resp, err = clus.sendOpWithResp(lead, rpcpb.Operation_FETCH_SNAPSHOT)
		clus.lg.Info(
			"install snapshot on leader node END",
			zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
			zap.Error(err),
		)
		return err
	}
	resp, err = clus.sendOpWithResp(lead, rpcpb.Operation_FETCH_SNAPSHOT)
	clus.lg.Info(
		"install snapshot on leader node END",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
		zap.String("member-name", resp.SnapshotInfo.MemberName),
		zap.Strings("member-client-urls", resp.SnapshotInfo.MemberClientURLs),
		zap.String("snapshot-path", resp.SnapshotInfo.SnapshotPath),
		zap.String("snapshot-file-size", resp.SnapshotInfo.SnapshotFileSize),
		zap.String("snapshot-total-size", resp.SnapshotInfo.SnapshotTotalSize),
		zap.Int64("snapshot-total-key", resp.SnapshotInfo.SnapshotTotalKey),
		zap.Int64("snapshot-hash", resp.SnapshotInfo.SnapshotHash),
		zap.Int64("snapshot-revision", resp.SnapshotInfo.SnapshotRevision),
		zap.String("took", resp.SnapshotInfo.Took),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	leaderc, err := clus.Members[lead].CreateEtcdClient()
	if err != nil {
		return err
	}
	defer leaderc.Close()
	var mresp *clientv3.MemberListResponse
	mresp, err = leaderc.MemberList(context.Background())
	mss := []string{}
	if err == nil && mresp != nil {
		mss = describeMembers(mresp)
	}
	clus.lg.Info(
		"member list before disastrous machine failure",
		zap.String("request-to", clus.Members[lead].EtcdClientEndpoint),
		zap.Strings("members", mss),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	// simulate real life; machine failures may happen
	// after some time since last snapshot save
	time.Sleep(time.Second)

	//  3. Destroy node A and B, and make the whole cluster inoperable.
	for {
		f.injected = pickQuorum(len(clus.Members))
		if _, ok := f.injected[lead]; !ok {
			break
		}
	}
	for idx := range f.injected {
		clus.lg.Info(
			"disastrous machine failure to quorum START",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
			zap.Error(err),
		)
		err = clus.sendOp(idx, rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA)
		clus.lg.Info(
			"disastrous machine failure to quorum END",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
			zap.Error(err),
		)
		if err != nil {
			return err
		}
	}

	//  4. Now node C cannot operate either.
	//  5. SIGTERM node C and remove its data directories.
	clus.lg.Info(
		"disastrous machine failure to old leader START",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
		zap.Error(err),
	)
	err = clus.sendOp(lead, rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA)
	clus.lg.Info(
		"disastrous machine failure to old leader END",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
		zap.Error(err),
	)
	return err
}

func (f *fetchSnapshotAndFailureQuorum) Recover(clus *Cluster) error {
	//  6. Restore a new seed member from node C's latest snapshot file.

	//  7. Add another member to establish 2-node cluster.

	//  8. Add another member to establish 3-node cluster.

	// for idx := range f.injected {
	// 	if err := f.recoverMember(clus, idx); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

func (f *fetchSnapshotAndFailureQuorum) Desc() string {
	if f.desc != "" {
		return f.desc
	}
	return f.failureCase.String()
}

func (f *fetchSnapshotAndFailureQuorum) FailureCase() rpcpb.FailureCase {
	return f.failureCase
}

func new_FailureCase_SIGQUIT_AND_REMOVE_QUORUM_AND_RESTORE_LEADER_SNAPSHOT_FROM_SCRATCH(clus *Cluster) Failure {
	f := &fetchSnapshotAndFailureQuorum{
		failureCase: rpcpb.FailureCase_SIGQUIT_AND_REMOVE_QUORUM_AND_RESTORE_LEADER_SNAPSHOT_FROM_SCRATCH,
		injected:    make(map[int]struct{}),
		snapshotted: -1,
	}
	// simulate real life; machine replacements may happen
	// after some time since disaster
	return &failureDelay{
		Failure:       f,
		delayDuration: clus.GetFailureDelayDuration(),
	}
}
