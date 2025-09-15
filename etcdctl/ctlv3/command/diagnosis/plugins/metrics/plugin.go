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

package metrics

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/engine/intf"
	"go.etcd.io/etcd/etcdctl/v3/ctlv3/command/diagnosis/plugins/common"
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

func NewPlugin(cfg *clientv3.ConfigSpec, eps []string, timeout time.Duration) intf.Plugin {
	return &metricsChecker{
		Checker: common.Checker{
			Cfg:            cfg,
			Endpoints:      eps,
			CommandTimeout: timeout,
			Name:           "metricsChecker",
		},
	}
}

func (ck *metricsChecker) Name() string {
	return ck.Checker.Name
}

func (ck *metricsChecker) Diagnose() (result any) {
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

	chkResult := checkResult{
		Name:          ck.Name(),
		Summary:       []string{},
		EpMetricsList: make([]epMetrics, len(eps)),
	}

	for i, ep := range eps {
		chkResult.EpMetricsList[i].Endpoint = ep

		startTs := time.Now()
		allMetrics, err := fetchMetrics(ck.Cfg, ep, ck.CommandTimeout)
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
	return result
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

func fetchMetrics(cfg *clientv3.ConfigSpec, ep string, timeout time.Duration) ([]string, error) {
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "http://" + ep
	}
	urlPath, err := url.JoinPath(ep, "metrics")
	if err != nil {
		return nil, fmt.Errorf("failed to join metrics url path: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	if strings.HasPrefix(urlPath, "https://") && cfg.Secure != nil {
		cert, err := tls.LoadX509KeyPair(cfg.Secure.Cert, cfg.Secure.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
		caCert, err := os.ReadFile(cfg.Secure.Cacert)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: cfg.Secure.InsecureSkipVerify,
			},
		}
		client.Transport = tr
	}
	resp, err := client.Get(urlPath)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics response: %w", err)
	}

	return strings.Split(string(data), "\n"), nil
}
