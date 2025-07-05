package membership

import (
	"log"
	"reflect"

	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/agent"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/common"
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

func NewPlugin(gcfg agent.GlobalConfig) intf.Plugin {
	return &membershipChecker{
		Checker: common.Checker{
			GlobalConfig: gcfg,
			Name:         "membershipChecker",
		},
	}
}

func (ck *membershipChecker) Name() string {
	return ck.Checker.Name
}

func (ck *membershipChecker) Diagnose() (result any) {
	var (
		eps []string
		err error
	)

	defer func() {
		if err != nil {
			result = &intf.FailedResult{
				Name:   ck.Name(),
				Reason: err.Error(),
			}
		}
	}()

	if eps, err = agent.Endpoints(ck.GlobalConfig); err != nil {
		log.Printf("Failed to get endpoint: %v\n", err)
		return
	}
	log.Printf("Endpoints: %v\n", eps)

	memberLists := make([]*clientv3.MemberListResponse, len(eps))
	detectedInconsistency := false
	for i, ep := range eps {
		if memberLists[i], err = agent.MemberList(ck.GlobalConfig, []string{ep}, clientv3.WithSerializable()); err != nil {
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

	return
}

func compareMembers(m1, m2 *clientv3.MemberListResponse) bool {
	if m1 == nil || m2 == nil {
		return false
	}

	return m1.Header.ClusterId == m2.Header.ClusterId && reflect.DeepEqual(m1.Members, m2.Members)
}
