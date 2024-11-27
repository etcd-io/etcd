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
	"time"

	"github.com/stretchr/testify/require"
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
	path, err := filepath.Abs(filepath.Join(
		resultsDirectory, strings.ReplaceAll(t.Name(), "/", "_"), fmt.Sprintf("%v", time.Now().UnixNano())))
	require.NoError(t, err)
	err = os.RemoveAll(path)
	require.NoError(t, err)
	err = os.MkdirAll(path, 0o700)
	require.NoError(t, err)
	return path
}

func (r *TestReport) Report(t *testing.T, force bool) {
	_, persistResultsEnvSet := os.LookupEnv("PERSIST_RESULTS")
	if !t.Failed() && !force && !persistResultsEnvSet {
		return
	}
	path := testResultsDirectory(t)
	r.Logger.Info("Saving robustness test report", zap.String("path", path))
	for _, member := range r.Cluster.Procs {
		memberDataDir := filepath.Join(path, fmt.Sprintf("server-%s", member.Config().Name))
		persistMemberDataDir(t, r.Logger, member, memberDataDir)
	}
	if r.Client != nil {
		persistClientReports(t, r.Logger, path, r.Client)
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
	err := os.Rename(memberDataDir(member), path)
	require.NoError(t, err)
}

func memberDataDir(member e2e.EtcdProcess) string {
	lazyFS := member.LazyFS()
	if lazyFS != nil {
		return filepath.Join(lazyFS.LazyFSDir, "data")
	}
	return member.Config().DataDirPath
}
