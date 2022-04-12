// Copyright 2022 The etcd Authors
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

package framework

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

type unitRunner struct{}

var _ testRunner = (*unitRunner)(nil)

func (e unitRunner) TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Short() {
		fmt.Println(`No test mode selected, please selected either e2e mode with "--tags e2e" or integration mode with "--tags integration"`)
		os.Exit(1)
	}
}

func (e unitRunner) BeforeTest(t testing.TB) {
}

func (e unitRunner) NewCluster(t testing.TB, cfg config.ClusterConfig) Cluster {
	testutil.SkipTestIfShortMode(t, "Cannot create clusters in --short tests")
	return nil
}
