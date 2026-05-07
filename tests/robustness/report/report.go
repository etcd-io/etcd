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

func NewTestReport(lg *zap.Logger, reportPath string, serverDataPaths map[string]string, traffic *TrafficDetail) *TestReport {
	return &TestReport{
		logger:          lg,
		reportPath:      reportPath,
		serverDataPaths: serverDataPaths,
		traffic:         traffic,
	}
}

func (r *TestReport) SetClientReports(reports []ClientReport) {
	r.clientReports = reports
}

func (r *TestReport) SetVisualize(visualize func(lg *zap.Logger, path string) error) {
	r.visualize = visualize
}

func (r *TestReport) ReportData() error {
	path, err := r.reportPathOrError()
	if err != nil {
		return err
	}
	r.logger.Info("Saving robustness test report", zap.String("path", path))
	err = os.RemoveAll(path)
	if err != nil {
		r.logger.Error("Failed to remove report dir", zap.Error(err))
	}
	for server, dataPath := range r.serverDataPaths {
		serverReportPath := filepath.Join(path, fmt.Sprintf("server-%s", server))
		r.logger.Info("Saving member data dir", zap.String("member", server), zap.String("data-dir", dataPath), zap.String("path", serverReportPath))
		if err := os.CopyFS(serverReportPath, os.DirFS(dataPath)); err != nil {
			return err
		}
	}
	if r.clientReports != nil {
		if err := persistClientReports(r.logger, path, r.clientReports); err != nil {
			return err
		}
	}
	if r.traffic != nil {
		if err := persistTrafficDetail(r.logger, path, *r.traffic); err != nil {
			return err
		}
	}
	r.dataSaved = true
	return nil
}

func (r *TestReport) ReportVisualization() error {
	if r.visualize == nil {
		return fmt.Errorf("cannot save report visualization: visualizer is not set")
	}
	path, err := r.reportPathOrError()
	if err != nil {
		return err
	}
	return r.visualize(r.logger, filepath.Join(path, "history.html"))
}

func (r *TestReport) Finalize(keep bool) error {
	if !r.dataSaved {
		return nil
	}
	path, err := r.reportPathOrError()
	if err != nil {
		return err
	}
	if !keep {
		r.logger.Info("Removing robustness test report", zap.String("path", path))
		return os.RemoveAll(path)
	}
	r.logger.Info("Keeping robustness test report", zap.String("path", path))
	if r.visualize == nil {
		r.logger.Info("No visualization available to be saved", zap.String("path", path))
		return nil
	}
	r.logger.Info("Adding visualization to test report", zap.String("path", path))
	return r.ReportVisualization()
}

func (r *TestReport) reportPathOrError() (string, error) {
	if r.reportPath == "" {
		return "", fmt.Errorf("report path is not set")
	}
	return r.reportPath, nil
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
