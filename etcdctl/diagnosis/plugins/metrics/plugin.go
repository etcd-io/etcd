package metrics

import (
	"fmt"
	"log"
	"strings"
	"time"

	"go.etcd.io/etcd/etcdctl/v3/diagnosis/agent"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/diagnosis/plugins/common"
)

var metricsNames = []string{
	"etcd_disk_wal_fsync_duration_seconds_bucket",
	"etcd_disk_backend_commit_duration_seconds_bucket",
	"etcd_network_peer_round_trip_time_seconds_bucket",
	"process_resident_memory_bytes",
	//"process_cpu_seconds_total",
}

type metricsChecker struct {
	common.Checker
}

type epMetrics struct {
	Endpoint  string              `json:"endpoint,omitempty"`
	Took      string              `json:"took,omitempty"`
	EpMetrics map[string][]string `json:"epMetrics,omitempty"`
}

type checkResult struct {
	Name          string      `json:"name,omitempty"`
	Summary       []string    `json:"summary,omitempty"`
	EpMetricsList []epMetrics `json:"epMetricsList,omitempty"`
}

func NewPlugin(gcfg agent.GlobalConfig) intf.Plugin {
	return &metricsChecker{
		Checker: common.Checker{
			GlobalConfig: gcfg,
			Name:         "metricsChecker",
		},
	}
}

func (ck *metricsChecker) Name() string {
	return ck.Checker.Name
}

func (ck *metricsChecker) Diagnose() (result any) {
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
		chkResult = checkResult{
			Name:          ck.Name(),
			Summary:       []string{},
			EpMetricsList: make([]epMetrics, len(eps)),
		}
	)

	for i, ep := range eps {
		chkResult.EpMetricsList[i].Endpoint = ep

		startTs := time.Now()
		allMetrics, err := agent.Metrics(ck.GlobalConfig, ep)
		chkResult.EpMetricsList[i].Took = time.Since(startTs).String()
		if err != nil {
			appendSummary(&chkResult, "Failed to get endpoint metrics from %q: %v", ep, err)
			continue
		}

		metricsMap := map[string][]string{}
		for _, prefix := range metricsNames {
			ret := metrics(allMetrics, prefix)
			metricsMap[prefix] = ret
		}

		chkResult.EpMetricsList[i].EpMetrics = metricsMap
	}

	if len(chkResult.Summary) == 0 {
		chkResult.Summary = []string{"Successful"}
	}

	result = chkResult
	return
}

func metrics(lines []string, prefix string) []string {
	var ret []string
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			ret = append(ret, line)
		}
	}
	return ret
}

func appendSummary(chkResult *checkResult, format string, v ...any) {
	errMsg := fmt.Sprintf(format, v...)
	log.Println(errMsg)
	chkResult.Summary = append(chkResult.Summary, errMsg)
}
