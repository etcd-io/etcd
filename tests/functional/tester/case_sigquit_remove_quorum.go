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
	"fmt"
	"strings"
	"time"

	"go.etcd.io/etcd/tests/v3/functional/rpcpb"
	"go.etcd.io/etcd/v3/clientv3"

	"go.uber.org/zap"
)

type fetchSnapshotCaseQuorum struct {
	desc        string
	rpcpbCase   rpcpb.Case
	injected    map[int]struct{}
	snapshotted int
}

func (c *fetchSnapshotCaseQuorum) Inject(clus *Cluster) error {
	// 1. Assume node C is the current leader with most up-to-date data.
	lead, err := clus.GetLeader()
	if err != nil {
		return err
	}
	c.snapshotted = lead

	// 2. Download snapshot from node C, before destroying node A and B.
	clus.lg.Info(
		"save snapshot on leader node START",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
	)
	var resp *rpcpb.Response
	resp, err = clus.sendOpWithResp(lead, rpcpb.Operation_SAVE_SNAPSHOT)
	if resp == nil || (resp != nil && !resp.Success) || err != nil {
		clus.lg.Info(
			"save snapshot on leader node FAIL",
			zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
			zap.Error(err),
		)
		return err
	}
	clus.lg.Info(
		"save snapshot on leader node SUCCESS",
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
	clus.Members[lead].SnapshotInfo = resp.SnapshotInfo

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

	// 3. Destroy node A and B, and make the whole cluster inoperable.
	for {
		c.injected = pickQuorum(len(clus.Members))
		if _, ok := c.injected[lead]; !ok {
			break
		}
	}
	for idx := range c.injected {
		clus.lg.Info(
			"disastrous machine failure to quorum START",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
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

	// 4. Now node C cannot operate either.
	// 5. SIGTERM node C and remove its data directories.
	clus.lg.Info(
		"disastrous machine failure to old leader START",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
	)
	err = clus.sendOp(lead, rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA)
	clus.lg.Info(
		"disastrous machine failure to old leader END",
		zap.String("target-endpoint", clus.Members[lead].EtcdClientEndpoint),
		zap.Error(err),
	)
	return err
}

func (c *fetchSnapshotCaseQuorum) Recover(clus *Cluster) error {
	// 6. Restore a new seed member from node C's latest snapshot file.
	oldlead := c.snapshotted

	// configuration on restart from recovered snapshot
	// seed member's configuration is all the same as previous one
	// except initial cluster string is now a single-node cluster
	clus.Members[oldlead].EtcdOnSnapshotRestore = clus.Members[oldlead].Etcd
	clus.Members[oldlead].EtcdOnSnapshotRestore.InitialClusterState = "existing"
	name := clus.Members[oldlead].Etcd.Name
	initClus := []string{}
	for _, u := range clus.Members[oldlead].Etcd.AdvertisePeerURLs {
		initClus = append(initClus, fmt.Sprintf("%s=%s", name, u))
	}
	clus.Members[oldlead].EtcdOnSnapshotRestore.InitialCluster = strings.Join(initClus, ",")

	clus.lg.Info(
		"restore snapshot and restart from snapshot request START",
		zap.String("target-endpoint", clus.Members[oldlead].EtcdClientEndpoint),
		zap.Strings("initial-cluster", initClus),
	)
	err := clus.sendOp(oldlead, rpcpb.Operation_RESTORE_RESTART_FROM_SNAPSHOT)
	clus.lg.Info(
		"restore snapshot and restart from snapshot request END",
		zap.String("target-endpoint", clus.Members[oldlead].EtcdClientEndpoint),
		zap.Strings("initial-cluster", initClus),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	leaderc, err := clus.Members[oldlead].CreateEtcdClient()
	if err != nil {
		return err
	}
	defer leaderc.Close()

	// 7. Add another member to establish 2-node cluster.
	// 8. Add another member to establish 3-node cluster.
	// 9. Add more if any.
	idxs := make([]int, 0, len(c.injected))
	for idx := range c.injected {
		idxs = append(idxs, idx)
	}
	clus.lg.Info("member add START", zap.Int("members-to-add", len(idxs)))
	for i, idx := range idxs {
		clus.lg.Info(
			"member add request SENT",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
			zap.Strings("peer-urls", clus.Members[idx].Etcd.AdvertisePeerURLs),
		)
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		_, err := leaderc.MemberAdd(ctx, clus.Members[idx].Etcd.AdvertisePeerURLs)
		cancel()
		clus.lg.Info(
			"member add request DONE",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
			zap.Strings("peer-urls", clus.Members[idx].Etcd.AdvertisePeerURLs),
			zap.Error(err),
		)
		if err != nil {
			return err
		}

		// start the added(new) member with fresh data
		clus.Members[idx].EtcdOnSnapshotRestore = clus.Members[idx].Etcd
		clus.Members[idx].EtcdOnSnapshotRestore.InitialClusterState = "existing"
		name := clus.Members[idx].Etcd.Name
		for _, u := range clus.Members[idx].Etcd.AdvertisePeerURLs {
			initClus = append(initClus, fmt.Sprintf("%s=%s", name, u))
		}
		clus.Members[idx].EtcdOnSnapshotRestore.InitialCluster = strings.Join(initClus, ",")
		clus.lg.Info(
			"restart from snapshot request SENT",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
			zap.Strings("initial-cluster", initClus),
		)
		err = clus.sendOp(idx, rpcpb.Operation_RESTART_FROM_SNAPSHOT)
		clus.lg.Info(
			"restart from snapshot request DONE",
			zap.String("target-endpoint", clus.Members[idx].EtcdClientEndpoint),
			zap.Strings("initial-cluster", initClus),
			zap.Error(err),
		)
		if err != nil {
			return err
		}

		if i != len(c.injected)-1 {
			// wait until membership reconfiguration entry gets applied
			// TODO: test concurrent member add
			dur := 5 * clus.Members[idx].ElectionTimeout()
			clus.lg.Info(
				"waiting after restart from snapshot request",
				zap.Int("i", i),
				zap.Int("idx", idx),
				zap.Duration("sleep", dur),
			)
			time.Sleep(dur)
		} else {
			clus.lg.Info(
				"restart from snapshot request ALL END",
				zap.Int("i", i),
				zap.Int("idx", idx),
			)
		}
	}
	return nil
}

func (c *fetchSnapshotCaseQuorum) Desc() string {
	if c.desc != "" {
		return c.desc
	}
	return c.rpcpbCase.String()
}

func (c *fetchSnapshotCaseQuorum) TestCase() rpcpb.Case {
	return c.rpcpbCase
}

func new_Case_SIGQUIT_AND_REMOVE_QUORUM_AND_RESTORE_LEADER_SNAPSHOT_FROM_SCRATCH(clus *Cluster) Case {
	c := &fetchSnapshotCaseQuorum{
		rpcpbCase:   rpcpb.Case_SIGQUIT_AND_REMOVE_QUORUM_AND_RESTORE_LEADER_SNAPSHOT_FROM_SCRATCH,
		injected:    make(map[int]struct{}),
		snapshotted: -1,
	}
	// simulate real life; machine replacements may happen
	// after some time since disaster
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}
