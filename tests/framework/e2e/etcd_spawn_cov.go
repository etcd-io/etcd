// Copyright 2017 The etcd Authors
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

//go:build cov
// +build cov

package e2e

import (
	"fmt"
	"os"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

const noOutputLineCount = 2 // cov-enabled binaries emit PASS and coverage count lines

var (
	coverDir = testutils.MustAbsPath(os.Getenv("COVERDIR"))
)

func init() {
	initBinPath = initBinPathCov
	additionalArgs = additionalArgsCov
}

func initBinPathCov(binDir string) binPath {
	return binPath{
		Etcd:            binDir + "/etcd_test",
		EtcdLastRelease: binDir + "/etcd-last-release",
		Etcdctl:         binDir + "/etcdctl_test",
		Etcdutl:         binDir + "/etcdutl_test",
	}
}

func additionalArgsCov() ([]string, error) {
	if !fileutil.Exist(coverDir) {
		return nil, fmt.Errorf("could not find coverage folder: %s", coverDir)
	}
	covArgs := []string{
		fmt.Sprintf("-test.coverprofile=e2e.%v.coverprofile", time.Now().UnixNano()),
		"-test.outputdir=" + coverDir,
	}
	return covArgs, nil
}
