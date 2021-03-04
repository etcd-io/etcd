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
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"go.etcd.io/etcd/pkg/v3/expect"
	"go.etcd.io/etcd/pkg/v3/fileutil"
)

const noOutputLineCount = 2 // cov-enabled binaries emit PASS and coverage count lines

func spawnCmd(args []string) (*expect.ExpectProcess, error) {
	cmd := args[0]
	env := make([]string, 0)
	switch cmd {
	case binPath:
		cmd = "../../bin/etcd_test"
	case ctlBinPath:
		cmd = "../../bin/etcdctl_test"
	case ctlBinPath + "3":
		cmd = "../../bin/etcdctl_test"
		env = append(env, "ETCDCTL_API=3")
	}

	covArgs, err := getCovArgs()
	if err != nil {
		return nil, err
	}
	// when withFlagByEnv() is used in testCtl(), env variables for ctl is set to os.env.
	// they must be included in ctl_cov_env.
	env = append(env, os.Environ()...)
	all_args := append(args[1:], covArgs...)
	log.Printf("Executing %v %v", cmd, all_args)
	ep, err := expect.NewExpectWithEnv(cmd, all_args, env)
	if err != nil {
		return nil, err
	}
	ep.StopSignal = syscall.SIGTERM
	return ep, nil
}

func getCovArgs() ([]string, error) {
	coverPath := os.Getenv("COVERDIR")
	if !filepath.IsAbs(coverPath) {
		// COVERDIR is relative to etcd root but e2e test has its path set to be relative to the e2e folder.
		// adding ".." in front of COVERDIR ensures that e2e saves coverage reports to the correct location.
		coverPath = filepath.Join("../..", coverPath)
	}
	if !fileutil.Exist(coverPath) {
		return nil, fmt.Errorf("could not find coverage folder")
	}
	covArgs := []string{
		fmt.Sprintf("-test.coverprofile=e2e.%v.coverprofile", time.Now().UnixNano()),
		"-test.outputdir=" + coverPath,
	}
	return covArgs, nil
}
