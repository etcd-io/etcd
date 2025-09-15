// Copyright 2025 The etcd Authors
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

package membership

import (
	"context"
	"log"
	"reflect"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/common"
)

type membershipChecker struct {
	common.Checker
}

type checkResult struct {
	Name           string                         `json:"name,omitempty"`
	Summary        string                         `json:"summary,omitempty"`
	MemberList     *clientv3.MemberListResponse   `json:"memberList,omitempty"`
	AllMemberLists []*clientv3.MemberListResponse `json:"allMemberLists,omitempty"`
}

func NewPlugin(cfg *clientv3.ConfigSpec, eps []string, timeout time.Duration) intf.Plugin {
	return &membershipChecker{
		Checker: common.Checker{
			Cfg:            cfg,
			Endpoints:      eps,
			CommandTimeout: timeout,
			Name:           "membershipChecker",
		},
	}
}

func (ck *membershipChecker) Name() string {
	return ck.Checker.Name
}

func (ck *membershipChecker) Diagnose() (result any) {
	var err error
	eps := ck.Endpoints

	defer func() {
		if err != nil {
			result = &intf.FailedResult{
				Name:   ck.Name(),
				Reason: err.Error(),
			}
		}
	}()

	memberLists := make([]*clientv3.MemberListResponse, len(eps))
	detectedInconsistency := false
	for i, ep := range eps {
		cfg := common.ConfigWithEndpoint(ck.Cfg, ep)
		c, err := common.NewClient(cfg)
		if err != nil {
			detectedInconsistency = true
			log.Printf("Failed to create client for %q: %v\n", ep, err)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), ck.CommandTimeout)
		memberLists[i], err = c.MemberList(ctx, clientv3.WithSerializable())
		cancel()
		c.Close()
		if err != nil {
			detectedInconsistency = true
			log.Printf("Failed to get member list from %q: %v\n", ep, err)
			continue
		}

		if i > 0 {
			if !compareMembers(memberLists[0], memberLists[i]) {
				detectedInconsistency = true
				log.Printf("Detected inconsistent member list between %q and %q\n", eps[0], eps[i])
			}
		}
	}

	if detectedInconsistency {
		result = checkResult{
			Name:           ck.Name(),
			Summary:        "Detected inconsistent member list between different members",
			AllMemberLists: memberLists,
		}
	} else {
		result = checkResult{
			Name:       ck.Name(),
			Summary:    "Successful",
			MemberList: memberLists[0],
		}
	}

	return result
}

func compareMembers(m1, m2 *clientv3.MemberListResponse) bool {
	if m1 == nil || m2 == nil {
		return false
	}

	return m1.Header.ClusterId == m2.Header.ClusterId && reflect.DeepEqual(m1.Members, m2.Members)
}
