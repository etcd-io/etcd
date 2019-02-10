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

// +build cov

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.etcd.io/etcd/pkg/expect"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/pkg/flags"
)

const noOutputLineCount = 2 // cov-enabled binaries emit PASS and coverage count lines

func spawnCmd(args []string) (*expect.ExpectProcess, error) {
	if args[0] == binPath {
		return spawnEtcd(args)
	}
	if args[0] == ctlBinPath || args[0] == ctlBinPath+"3" {
		// avoid test flag conflicts in coverage enabled etcdctl by putting flags in ETCDCTL_ARGS
		env := []string{
			// was \xff, but that's used for testing boundary conditions; 0xe7cd should be safe
			"ETCDCTL_ARGS=" + strings.Join(args, "\xe7\xcd"),
		}
		if args[0] == ctlBinPath+"3" {
			env = append(env, "ETCDCTL_API=3")
		}

		covArgs, err := getCovArgs()
		if err != nil {
			return nil, err
		}
		// when withFlagByEnv() is used in testCtl(), env variables for ctl is set to os.env.
		// they must be included in ctl_cov_env.
		env = append(env, os.Environ()...)
		ep, err := expect.NewExpectWithEnv(binDir+"/etcdctl_test", covArgs, env)
		if err != nil {
			return nil, err
		}
		ep.StopSignal = syscall.SIGTERM
		return ep, nil
	}

	return expect.NewExpect(args[0], args[1:]...)
}

func spawnEtcd(args []string) (*expect.ExpectProcess, error) {
	covArgs, err := getCovArgs()
	if err != nil {
		return nil, err
	}

	var env []string
	if args[1] == "grpc-proxy" {
		// avoid test flag conflicts in coverage enabled etcd by putting flags in ETCDCOV_ARGS
		env = append(os.Environ(), "ETCDCOV_ARGS="+strings.Join(args, "\xe7\xcd"))
	} else {
		env = args2env(args[1:])
	}

	ep, err := expect.NewExpectWithEnv(binDir+"/etcd_test", covArgs, env)
	if err != nil {
		return nil, err
	}
	// ep sends SIGTERM to etcd_test process on ep.close()
	// allowing the process to exit gracefully in order to generate a coverage report.
	// note: go runtime ignores SIGINT but not SIGTERM
	// if e2e test is run as a background process.
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

func args2env(args []string) []string {
	var covEnvs []string
	for i := range args {
		if !strings.HasPrefix(args[i], "--") {
			continue
		}
		flag := strings.Split(args[i], "--")[1]
		val := "true"
		// split the flag that has "="
		// e.g --auto-tls=true" => flag=auto-tls and val=true
		if strings.Contains(args[i], "=") {
			split := strings.Split(flag, "=")
			flag = split[0]
			val = split[1]
		}

		if i+1 < len(args) {
			if !strings.HasPrefix(args[i+1], "--") {
				val = args[i+1]
			}
		}
		covEnvs = append(covEnvs, flags.FlagToEnv("ETCD", flag)+"="+val)
	}
	return covEnvs
}
