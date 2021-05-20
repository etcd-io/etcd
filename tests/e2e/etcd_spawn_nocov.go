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

//go:build !cov
// +build !cov

package e2e

import (
	"os"
	"strings"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.uber.org/zap"
)

const noOutputLineCount = 0 // regular binaries emit no extra lines

func spawnCmd(args []string) (*expect.ExpectProcess, error) {
	return spawnCmdWithLogger(zap.NewNop(), args)
}

func spawnCmdWithLogger(lg *zap.Logger, args []string) (*expect.ExpectProcess, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(args[0], "/etcdctl3") {
		env := append(os.Environ(), "ETCDCTL_API=3")
		lg.Info("spawning process with ETCDCTL_API=3", zap.Strings("args", args), zap.String("working-dir", wd))
		return expect.NewExpectWithEnv(ctlBinPath, args[1:], env)
	}
	lg.Info("spawning process", zap.Strings("args", args), zap.String("working-dir", wd))
	return expect.NewExpect(args[0], args[1:]...)
}
