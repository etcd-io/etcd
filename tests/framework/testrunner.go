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
	"go.etcd.io/etcd/tests/v3/framework/e2e"
	"go.etcd.io/etcd/tests/v3/framework/integration"
	intf "go.etcd.io/etcd/tests/v3/framework/interfaces"
	"go.etcd.io/etcd/tests/v3/framework/unit"
)

var (
	// UnitTestRunner only runs in `--short` mode, will fail otherwise. Attempts in cluster creation will result in tests being skipped.
	UnitTestRunner intf.TestRunner = unit.NewUnitRunner()
	// E2eTestRunner runs etcd and etcdctl binaries in a separate process.
	E2eTestRunner = e2e.NewE2eRunner()
	// IntegrationTestRunner runs etcdserver.EtcdServer in separate goroutine and uses client libraries to communicate.
	IntegrationTestRunner = integration.NewIntegrationRunner()
)
