// Copyright 2018 The etcd Authors
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

// etcd-tester is a program that runs functional-tester client.
package main

import (
	"flag"
	"testing"

	"go.etcd.io/etcd/client/pkg/v3/testutil"
	"go.etcd.io/etcd/tests/v3/functional/tester"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

var config = flag.String("config", "../../functional.yaml", "path to tester configuration")

func TestFunctional(t *testing.T) {
	testutil.SkipTestIfShortMode(t, "functional tests are skipped in --short mode")

	lg := zaptest.NewLogger(t, zaptest.Level(zapcore.InfoLevel)).Named("tester")

	clus, err := tester.NewCluster(lg, *config)
	if err != nil {
		t.Fatalf("failed to create a cluster: %v", err)
	}

	err = clus.Send_INITIAL_START_ETCD()
	if err != nil {
		t.Fatal("Bootstrap failed", zap.Error(err))
	}
	defer clus.Send_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT()

	t.Log("wait health after bootstrap")
	err = clus.WaitHealth()
	if err != nil {
		t.Fatal("WaitHealth failed", zap.Error(err))
	}

	clus.Run(t)
}
