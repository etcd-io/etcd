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
	"strings"
	"syscall"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/tests/v3/integration"
	"go.uber.org/zap"
)

const noOutputLineCount = 2 // cov-enabled binaries emit PASS and coverage count lines

var (
	coverDir = integration.MustAbsPath(os.Getenv("COVERDIR"))
)

func spawnCmd(args []string) (*expect.ExpectProcess, error) {
	return spawnCmdWithLogger(zap.NewNop(), args)
}

func spawnCmdWithLogger(lg *zap.Logger, args []string) (*expect.ExpectProcess, error) {
	cmd := args[0]
	env := make([]string, 0)
	switch {
	case strings.HasSuffix(cmd, "/etcd"):
		cmd = cmd + "_test"
	case strings.HasSuffix(cmd, "/etcdctl"):
		cmd = cmd + "_test"
	case strings.HasSuffix(cmd, "/etcdutl"):
		cmd = cmd + "_test"
	case strings.HasSuffix(cmd, "/etcdctl3"):
		cmd = ctlBinPath + "_test"
		env = append(env, "ETCDCTL_API=3")
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	covArgs, err := getCovArgs()
	if err != nil {
		return nil, err
	}
	// when withFlagByEnv() is used in testCtl(), env variables for ctl is set to os.env.
	// they must be included in ctl_cov_env.
	env = append(env, os.Environ()...)
	all_args := append(args[1:], covArgs...)
	lg.Info("spawning process", zap.Strings("args", all_args), zap.String("working-dir", wd))
	ep, err := expect.NewExpectWithEnv(cmd, all_args, env)
	if err != nil {
		return nil, err
	}
	ep.StopSignal = syscall.SIGTERM
	return ep, nil
}

func getCovArgs() ([]string, error) {
	if !fileutil.Exist(coverDir) {
		return nil, fmt.Errorf("could not find coverage folder: %s", coverDir)
	}
	covArgs := []string{
		fmt.Sprintf("-test.coverprofile=e2e.%v.coverprofile", time.Now().UnixNano()),
		"-test.outputdir=" + coverDir,
	}
	return covArgs, nil
}
