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

// etcd-agent is a program that runs functional-tester agent.
package main

import (
	"flag"

	"go.etcd.io/etcd/tests/v3/functional/agent"
	"go.uber.org/zap/zapcore"

	"go.uber.org/zap"
)

var logger *zap.Logger

func main() {
	network := flag.String("network", "tcp", "network to serve agent server")
	address := flag.String("address", "127.0.0.1:9027", "address to serve agent server")
	flag.Parse()

	lcfg := zap.NewDevelopmentConfig()
	lcfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := lcfg.Build()
	if err != nil {
		panic(err)
	}

	logger = logger.Named("agent").With(zap.String("address", *address))

	defer logger.Sync()

	srv := agent.NewServer(logger, *network, *address)
	err = srv.StartServe()
	logger.Info("agent exiting", zap.Error(err))
}
