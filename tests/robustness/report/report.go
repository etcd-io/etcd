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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

type TestReport struct {
	Logger    *zap.Logger
	Cluster   *e2e.EtcdProcessCluster
	Client    []ClientReport
	Visualize func(path string) error
}

func testResultsDirectory(t *testing.T) string {
	resultsDirectory, ok := os.LookupEnv("RESULTS_DIR")
	if !ok {
		resultsDirectory = "/tmp/"
	}
	resultsDirectory, err := filepath.Abs(resultsDirectory)
	if err != nil {
		panic(err)
	}
	path, err := filepath.Abs(filepath.Join(resultsDirectory, strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll(path)
	if err != nil {
		t.Fatal(err)
	}
	err = os.MkdirAll(path, 0700)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func (r *TestReport) Report(t *testing.T, force bool) {
	path := testResultsDirectory(t)
	if t.Failed() || force {
		for _, member := range r.Cluster.Procs {
			memberDataDir := filepath.Join(path, fmt.Sprintf("server-%s", member.Config().Name))
			persistMemberDataDir(t, r.Logger, member, memberDataDir)
		}
		if r.Client != nil {
			persistClientReports(t, r.Logger, path, r.Client)
		}
	}
	if r.Visualize != nil {
		err := r.Visualize(filepath.Join(path, "history.html"))
		if err != nil {
			t.Error(err)
		}
	}
}

func persistMemberDataDir(t *testing.T, lg *zap.Logger, member e2e.EtcdProcess, path string) {
	lg.Info("Saving member data dir", zap.String("member", member.Config().Name), zap.String("path", path))
	err := os.Rename(member.Config().DataDirPath, path)
	if err != nil {
		t.Fatal(err)
	}
}
