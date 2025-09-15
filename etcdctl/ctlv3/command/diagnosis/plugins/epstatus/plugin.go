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

package epstatus

import (
	"context"
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/common"
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

func NewPlugin(cfg *clientv3.ConfigSpec, eps []string, timeout time.Duration, dbQuota int) intf.Plugin {
	return &epStatusChecker{
		Checker: common.Checker{
			Cfg:            cfg,
			Endpoints:      eps,
			CommandTimeout: timeout,
			DbQuotaBytes:   dbQuota,
			Name:           "epStatusChecker",
		},
	}
}

func (ck *epStatusChecker) Name() string {
	return ck.Checker.Name
}

func (ck *epStatusChecker) Diagnose() (result any) {
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

	var (
		maxRetries  = 3
		retries     = 0
		shouldRetry = true

		chkResult = initCheckResult(ck.Name(), len(eps))
	)

	for {
		for i, ep := range eps {
			chkResult.EpStatusList[i].Endpoint = ep

			cfg := common.ConfigWithEndpoint(ck.Cfg, ep)
			c, err := common.NewClient(cfg)
			if err != nil {
				appendSummary(&chkResult, "Failed to create client for %q: %v", ep, err)
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), ck.CommandTimeout)
			chkResult.EpStatusList[i].EpStatus, err = c.Status(ctx, ep)
			cancel()
			c.Close()
			if err != nil {
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

	checkDBSize(&chkResult, ck.DbQuotaBytes)

	if len(chkResult.Summary) == 0 {
		chkResult.Summary = []string{"Successful"}
	}

	result = chkResult
	return result
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

func checkDBSize(chkResult *checkResult, dbQuota int) {
	for _, sts := range chkResult.EpStatusList {
		if sts.EpStatus == nil {
			continue
		}

		freeSize := sts.EpStatus.DbSize - sts.EpStatus.DbSizeInUse
		if freeSize > sts.EpStatus.DbSizeInUse && freeSize > 1_000_000_000 /* about 1GB */ || sts.EpStatus.DbSize >= int64(dbQuota*80/100) {
			appendSummary(chkResult, "Detected large amount of db [free] space for endpoint %q, dbQuota: %d, dbSize: %d, dbSizeInUse: %d, dbSizeFree: %d", sts.Endpoint, dbQuota, sts.EpStatus.DbSize, sts.EpStatus.DbSizeInUse, freeSize)
		}
	}
}
