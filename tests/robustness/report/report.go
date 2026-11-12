// Copyright 2023 The etcd Authors
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

package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

type TestReport struct {
	logger          *zap.Logger
	reportPath      string
	serverDataPaths map[string]string
	clientReports   []ClientReport
	visualize       func(lg *zap.Logger, path string) error
	traffic         *TrafficDetail
	dataSaved       bool
}

func NewTestReport(lg *zap.Logger, reportPath string, serverDataPaths map[string]string, traffic *TrafficDetail) (*TestReport, error) {
	if reportPath == "" {
		return nil, fmt.Errorf("reportPath is not set")
	}
	return &TestReport{
		logger:          lg,
		reportPath:      reportPath,
		serverDataPaths: serverDataPaths,
		traffic:         traffic,
	}, nil
}

func (r *TestReport) SetClientReports(reports []ClientReport) {
	r.clientReports = reports
}

func (r *TestReport) SetVisualizer(visualize func(lg *zap.Logger, path string) error) {
	r.visualize = visualize
}

func (r *TestReport) SaveEtcdData() error {
	if r.dataSaved {
		return nil
	}
	r.logger.Info("Saving etcd data", zap.String("path", r.reportPath))
	if err := os.MkdirAll(r.reportPath, 0o700); err != nil {
		return fmt.Errorf("failed to create report dir: %w", err)
	}
	for server, dataPath := range r.serverDataPaths {
		serverReportPath := filepath.Join(r.reportPath, fmt.Sprintf("server-%s", server))
		r.logger.Info("Saving member data dir", zap.String("member", server), zap.String("data-dir", dataPath), zap.String("path", serverReportPath))
		_ = os.RemoveAll(serverReportPath)
		if err := os.CopyFS(serverReportPath, os.DirFS(dataPath)); err != nil {
			return err
		}
	}
	existingReports, err := LoadClientReports(r.reportPath)
	if err != nil && !os.IsNotExist(err) {
		r.logger.Warn("Failed to load existing client reports", zap.Error(err))
	}
	r.clientReports = mergeClientReports(existingReports, r.clientReports)
	if len(r.clientReports) != 0 {
		if err := PersistClientReports(r.logger, r.reportPath, r.clientReports); err != nil {
			return err
		}
	}
	if r.traffic != nil {
		if err := persistTrafficDetail(r.logger, r.reportPath, *r.traffic); err != nil {
			return err
		}
	}
	r.dataSaved = true
	return nil
}

func mergeClientReports(existing, newReports []ClientReport) []ClientReport {
	if len(existing) == 0 {
		return newReports
	}
	if len(newReports) == 0 {
		return existing
	}
	reportsMap := make(map[int]ClientReport, len(existing)+len(newReports))
	for _, r := range existing {
		reportsMap[r.ClientID] = r
	}
	for _, r := range newReports {
		if ex, found := reportsMap[r.ClientID]; found {
			ex.KeyValue = append(ex.KeyValue, r.KeyValue...)
			ex.Watch = append(ex.Watch, r.Watch...)
			reportsMap[r.ClientID] = ex
		} else {
			reportsMap[r.ClientID] = r
		}
	}
	merged := make([]ClientReport, 0, len(reportsMap))
	for _, r := range reportsMap {
		merged = append(merged, r)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].ClientID < merged[j].ClientID
	})
	return merged
}

func (r *TestReport) Finalize(tFailed bool, panicked bool) error {
	_, persistResults := os.LookupEnv("PERSIST_RESULTS")
	keep := tFailed || panicked || persistResults
	if !keep {
		if !r.dataSaved {
			return nil
		}
		r.logger.Info("Removing robustness test report", zap.String("path", r.reportPath))
		return os.RemoveAll(r.reportPath)
	}

	if err := r.SaveEtcdData(); err != nil {
		return fmt.Errorf("failed to save etcd data: %w", err)
	}

	if r.visualize == nil {
		r.logger.Info("No visualization available to be saved", zap.String("path", r.reportPath))
		return nil
	}
	r.logger.Info("Adding visualization to test report", zap.String("path", r.reportPath))
	return r.visualize(r.logger, filepath.Join(r.reportPath, "history.html"))
}

func ServerDataPaths(c *e2e.EtcdProcessCluster) map[string]string {
	dataPaths := make(map[string]string)
	for _, member := range c.Procs {
		dataPaths[member.Config().Name] = memberDataDir(member)
	}

	return dataPaths
}

func memberDataDir(member e2e.EtcdProcess) string {
	lazyFS := member.LazyFS()
	if lazyFS != nil {
		return filepath.Join(lazyFS.LazyFSDir, "data")
	}
	return member.Config().DataDirPath
}

type TrafficDetail struct {
	ExpectUniqueRevision bool `json:"expectuniquerevision,omitempty"`
}

const trafficDetailFileName = "traffic.json"

func persistTrafficDetail(lg *zap.Logger, p string, td TrafficDetail) error {
	lg.Info("Saving traffic configuration details", zap.String("path", path.Join(p, trafficDetailFileName)))
	b, err := json.Marshal(td)
	if err != nil {
		return nil
	}
	return os.WriteFile(filepath.Join(p, trafficDetailFileName), b, 0o644)
}

func LoadTrafficDetail(p string) (TrafficDetail, error) {
	var detail TrafficDetail
	b, err := os.ReadFile(filepath.Join(p, trafficDetailFileName))
	if err != nil {
		return TrafficDetail{}, err
	}
	err = json.Unmarshal(b, &detail)
	return detail, err
}
