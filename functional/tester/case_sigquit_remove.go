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
	"sort"
	"strings"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/functional/rpcpb"

	"go.uber.org/zap"
)

func inject_SIGQUIT_ETCD_AND_REMOVE_DATA(clus *Cluster, idx1 int) error {
	cli1, err := clus.Members[idx1].CreateEtcdClient()
	if err != nil {
		return err
	}
	defer cli1.Close()

	var mresp *clientv3.MemberListResponse
	mresp, err = cli1.MemberList(context.Background())
	mss := []string{}
	if err == nil && mresp != nil {
		mss = describeMembers(mresp)
	}
	clus.lg.Info(
		"member list before disastrous machine failure",
		zap.String("request-to", clus.Members[idx1].EtcdClientEndpoint),
		zap.Strings("members", mss),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	sresp, serr := cli1.Status(context.Background(), clus.Members[idx1].EtcdClientEndpoint)
	if serr != nil {
		return serr
	}
	id1 := sresp.Header.MemberId
	is1 := fmt.Sprintf("%016x", id1)

	clus.lg.Info(
		"disastrous machine failure START",
		zap.String("target-endpoint", clus.Members[idx1].EtcdClientEndpoint),
		zap.String("target-member-id", is1),
		zap.Error(err),
	)
	err = clus.sendOp(idx1, rpcpb.Operation_SIGQUIT_ETCD_AND_REMOVE_DATA)
	clus.lg.Info(
		"disastrous machine failure END",
		zap.String("target-endpoint", clus.Members[idx1].EtcdClientEndpoint),
		zap.String("target-member-id", is1),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	idx2 := (idx1 + 1) % len(clus.Members)
	var cli2 *clientv3.Client
	cli2, err = clus.Members[idx2].CreateEtcdClient()
	if err != nil {
		return err
	}
	defer cli2.Close()

	// FIXME(bug): this may block forever during
	// "SIGQUIT_AND_REMOVE_LEADER_UNTIL_TRIGGER_SNAPSHOT"
	// is the new leader too busy with snapshotting?
	// is raft proposal dropped?
	// enable client keepalive for failover?
	clus.lg.Info(
		"member remove after disaster START",
		zap.String("target-endpoint", clus.Members[idx1].EtcdClientEndpoint),
		zap.String("target-member-id", is1),
		zap.String("request-to", clus.Members[idx2].EtcdClientEndpoint),
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	_, err = cli2.MemberRemove(ctx, id1)
	cancel()
	clus.lg.Info(
		"member remove after disaster END",
		zap.String("target-endpoint", clus.Members[idx1].EtcdClientEndpoint),
		zap.String("target-member-id", is1),
		zap.String("request-to", clus.Members[idx2].EtcdClientEndpoint),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	mresp, err = cli2.MemberList(context.Background())
	mss = []string{}
	if err == nil && mresp != nil {
		mss = describeMembers(mresp)
	}
	clus.lg.Info(
		"member list after member remove",
		zap.String("request-to", clus.Members[idx2].EtcdClientEndpoint),
		zap.Strings("members", mss),
		zap.Error(err),
	)
	return err
}

func recover_SIGQUIT_ETCD_AND_REMOVE_DATA(clus *Cluster, idx1 int) error {
	idx2 := (idx1 + 1) % len(clus.Members)
	cli2, err := clus.Members[idx2].CreateEtcdClient()
	if err != nil {
		return err
	}
	defer cli2.Close()

	_, err = cli2.MemberAdd(context.Background(), clus.Members[idx1].Etcd.AdvertisePeerURLs)
	clus.lg.Info(
		"member add before fresh restart",
		zap.String("target-endpoint", clus.Members[idx1].EtcdClientEndpoint),
		zap.String("request-to", clus.Members[idx2].EtcdClientEndpoint),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	clus.Members[idx1].Etcd.InitialClusterState = "existing"
	err = clus.sendOp(idx1, rpcpb.Operation_RESTART_ETCD)
	clus.lg.Info(
		"fresh restart after member add",
		zap.String("target-endpoint", clus.Members[idx1].EtcdClientEndpoint),
		zap.Error(err),
	)
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	var mresp *clientv3.MemberListResponse
	mresp, err = cli2.MemberList(context.Background())
	mss := []string{}
	if err == nil && mresp != nil {
		mss = describeMembers(mresp)
	}
	clus.lg.Info(
		"member list after member add",
		zap.String("request-to", clus.Members[idx2].EtcdClientEndpoint),
		zap.Strings("members", mss),
		zap.Error(err),
	)
	return err
}

func new_Case_SIGQUIT_AND_REMOVE_ONE_FOLLOWER(clus *Cluster) Case {
	cc := caseByFunc{
		rpcpbCase:     rpcpb.Case_SIGQUIT_AND_REMOVE_ONE_FOLLOWER,
		injectMember:  inject_SIGQUIT_ETCD_AND_REMOVE_DATA,
		recoverMember: recover_SIGQUIT_ETCD_AND_REMOVE_DATA,
	}
	c := &caseFollower{cc, -1, -1}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}

func new_Case_SIGQUIT_AND_REMOVE_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT(clus *Cluster) Case {
	return &caseUntilSnapshot{
		rpcpbCase: rpcpb.Case_SIGQUIT_AND_REMOVE_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT,
		Case:      new_Case_SIGQUIT_AND_REMOVE_ONE_FOLLOWER(clus),
	}
}

func new_Case_SIGQUIT_AND_REMOVE_LEADER(clus *Cluster) Case {
	cc := caseByFunc{
		rpcpbCase:     rpcpb.Case_SIGQUIT_AND_REMOVE_LEADER,
		injectMember:  inject_SIGQUIT_ETCD_AND_REMOVE_DATA,
		recoverMember: recover_SIGQUIT_ETCD_AND_REMOVE_DATA,
	}
	c := &caseLeader{cc, -1, -1}
	return &caseDelay{
		Case:          c,
		delayDuration: clus.GetCaseDelayDuration(),
	}
}

func new_Case_SIGQUIT_AND_REMOVE_LEADER_UNTIL_TRIGGER_SNAPSHOT(clus *Cluster) Case {
	return &caseUntilSnapshot{
		rpcpbCase: rpcpb.Case_SIGQUIT_AND_REMOVE_LEADER_UNTIL_TRIGGER_SNAPSHOT,
		Case:      new_Case_SIGQUIT_AND_REMOVE_LEADER(clus),
	}
}

func describeMembers(mresp *clientv3.MemberListResponse) (ss []string) {
	ss = make([]string, len(mresp.Members))
	for i, m := range mresp.Members {
		ss[i] = fmt.Sprintf("Name %s / ID %016x / ClientURLs %s / PeerURLs %s",
			m.Name,
			m.ID,
			strings.Join(m.ClientURLs, ","),
			strings.Join(m.PeerURLs, ","),
		)
	}
	sort.Strings(ss)
	return ss
}
