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
	Logger          *zap.Logger
	Cluster         *e2e.EtcdProcessCluster
	ServersDataPath map[string]string
	Client          []ClientReport
	Visualize       func(lg *zap.Logger, path string) error
	Traffic         *TrafficDetail
}

func (r *TestReport) Report(path string) error {
	r.Logger.Info("Saving robustness test report", zap.String("path", path))
	err := os.RemoveAll(path)
	if err != nil {
		r.Logger.Error("Failed to remove report dir", zap.Error(err))
	}
	for server, dataPath := range r.ServersDataPath {
		serverReportPath := filepath.Join(path, fmt.Sprintf("server-%s", server))
		r.Logger.Info("Saving member data dir", zap.String("member", server), zap.String("data-dir", dataPath), zap.String("path", serverReportPath))
		if err := os.CopyFS(serverReportPath, os.DirFS(dataPath)); err != nil {
			return err
		}
	}
	if r.Client != nil {
		if err := persistClientReports(r.Logger, path, r.Client); err != nil {
			return err
		}
	}
	if r.Traffic != nil {
		if err := persistTrafficDetail(r.Logger, path, *r.Traffic); err != nil {
			return err
		}
	}
	if r.Visualize != nil {
		if err := r.Visualize(r.Logger, filepath.Join(path, "history.html")); err != nil {
			return err
		}
	}
	return nil
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
