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
	"fmt"
	"os"
	"strings"

	"go.etcd.io/etcd/pkg/expect"
)

const noOutputLineCount = 0 // regular binaries emit no extra lines

func spawnCmd(args []string) (*expect.ExpectProcess, error) {
	return spawnCmdWithEnv(args, nil)
}

func spawnCmdWithEnv(args []string, envVars map[string]string) (*expect.ExpectProcess, error) {
	env := mergeEnvVariables(envVars)
	switch {
	case strings.HasSuffix(args[0], ctlBinPath+"2"):
		env = append(env, "ETCDCTL_API=2")
		args[0] = ctlBinPath
	case strings.HasSuffix(args[0], ctlBinPath+"3"):
		env = append(env, "ETCDCTL_API=3")
		args[0] = ctlBinPath
	}
	return expect.NewExpectWithEnv(args[0], args[1:], env)
}

func mergeEnvVariables(envVars map[string]string) []string {
	var env []string
	// Environment variables are passed as parameter have higher priority
	// than os environment variables.
	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Now, we can set os environment variables not passed as parameter.
	currVars := os.Environ()
	for _, v := range currVars {
		p := strings.Split(v, "=")
		if _, ok := envVars[p[0]]; !ok {
			env = append(env, fmt.Sprintf("%s=%s", p[0], p[1]))
		}
	}

	return env
}
