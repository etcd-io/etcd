package epstatus

import (
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/agent"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/common"
)

type epStatusChecker struct {
	common.Checker
}

type epStatus struct {
	Endpoint string                   `json:"endpoint,omitempty"`
	EpStatus *clientv3.StatusResponse `json:"epStatus,omitempty"`
}

type checkResult struct {
	Name         string     `json:"name,omitempty"`
	Summary      []string   `json:"summary,omitempty"`
	EpStatusList []epStatus `json:"epStatusList,omitempty"`
}

func NewPlugin(gcfg agent.GlobalConfig) intf.Plugin {
	return &epStatusChecker{
		Checker: common.Checker{
			GlobalConfig: gcfg,
			Name:         "epStatusChecker",
		},
	}
}

func (ck *epStatusChecker) Name() string {
	return ck.Checker.Name
}

func (ck *epStatusChecker) Diagnose() (result any) {
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

	var (
		maxRetries  = 3
		retries     = 0
		shouldRetry = true

		chkResult = initCheckResult(ck.Name(), len(eps))
	)

	for {
		for i, ep := range eps {
			chkResult.EpStatusList[i].Endpoint = ep
			if chkResult.EpStatusList[i].EpStatus, err = agent.EndpointStatus(ck.GlobalConfig, ep); err != nil {
				appendSummary(&chkResult, "Failed to get endpoint status from %q: %v", ep, err)
				continue
			}

			if len(chkResult.EpStatusList[i].EpStatus.Errors) > 0 {
				appendSummary(&chkResult, "Detected errors in endpoint %q: %v\n", ep, chkResult.EpStatusList[i].EpStatus.Errors)
				shouldRetry = false
				continue
			}

			if i > 0 {
				if !compareHardInfo(chkResult.EpStatusList[0].EpStatus, chkResult.EpStatusList[i].EpStatus) {
					appendSummary(&chkResult, "Detected inconsistent hard endpoint info between %q and %q\n", eps[0], eps[i])
					shouldRetry = false
				}

				if !shouldRetry {
					continue
				}

				if !compareSoftInfo(chkResult.EpStatusList[0].EpStatus, chkResult.EpStatusList[i].EpStatus) {
					appendSummary(&chkResult, "Detected inconsistent soft endpoint info between %q and %q\n", eps[0], eps[i])
				}
			}
		}

		retries++

		if len(chkResult.Summary) == 0 || !shouldRetry || retries >= maxRetries {
			break
		}

		chkResult = initCheckResult(ck.Name(), len(eps))
		log.Printf("Retrying checking endpoint status: %d/%d\n", retries, maxRetries)
		time.Sleep(time.Second)
	}

	checkDBSize(&chkResult, ck.GlobalConfig)

	if len(chkResult.Summary) == 0 {
		chkResult.Summary = []string{"Successful"}
	}

	result = chkResult
	return
}

func initCheckResult(name string, epCount int) checkResult {
	return checkResult{
		Name:         name,
		Summary:      []string{},
		EpStatusList: make([]epStatus, epCount),
	}
}

func appendSummary(chkResult *checkResult, format string, v ...any) {
	errMsg := fmt.Sprintf(format, v...)
	log.Println(errMsg)
	chkResult.Summary = append(chkResult.Summary, errMsg)
}

func compareHardInfo(s1, s2 *clientv3.StatusResponse) bool {
	if s1 == nil || s2 == nil {
		return false
	}
	return s1.Header.ClusterId == s2.Header.ClusterId &&
		s1.Version == s2.Version &&
		s1.StorageVersion == s2.StorageVersion
}

func compareSoftInfo(s1, s2 *clientv3.StatusResponse) bool {
	if s1 == nil || s2 == nil {
		return false
	}
	return s1.Header.Revision == s2.Header.Revision &&
		s1.RaftTerm == s2.RaftTerm &&
		s1.RaftIndex == s2.RaftIndex &&
		s1.RaftAppliedIndex == s2.RaftAppliedIndex &&
		s1.Leader == s2.Leader
}

func checkDBSize(chkResult *checkResult, gcfg agent.GlobalConfig) {
	for _, sts := range chkResult.EpStatusList {
		if sts.EpStatus == nil {
			continue
		}

		freeSize := sts.EpStatus.DbSize - sts.EpStatus.DbSizeInUse
		// TODO: allow users to define rules?
		if freeSize > sts.EpStatus.DbSizeInUse && freeSize > 1_000_000_000 /* about 1GB */ || sts.EpStatus.DbSize >= int64(gcfg.DbQuotaBytes*80/100) {
			appendSummary(chkResult, "Detected large amount of db [free] space for endpoint %q, dbQuota: %d, dbSize: %d, dbSizeInUse: %d, dbSizeFree: %d", sts.Endpoint, gcfg.DbQuotaBytes, sts.EpStatus.DbSize, sts.EpStatus.DbSizeInUse, freeSize)
		}
	}
}
