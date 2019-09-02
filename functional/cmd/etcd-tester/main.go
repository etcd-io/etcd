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

	"go.etcd.io/etcd/functional/tester"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	var err error
	logger, err = zap.NewProduction()
	if err != nil {
		panic(err)
	}
}

func main() {
	config := flag.String("config", "", "path to tester configuration")
	flag.Parse()

	defer logger.Sync()

	clus, err := tester.NewCluster(logger, *config)
	if err != nil {
		logger.Fatal("failed to create a cluster", zap.Error(err))
	}

	err = clus.Send_INITIAL_START_ETCD()
	if err != nil {
		logger.Fatal("Bootstrap failed", zap.Error(err))
	}
	defer clus.Send_SIGQUIT_ETCD_AND_REMOVE_DATA_AND_STOP_AGENT()

	logger.Info("wait health after bootstrap")
	err = clus.WaitHealth()
	if err != nil {
		logger.Fatal("WaitHealth failed", zap.Error(err))
	}

	clus.Run()
}
