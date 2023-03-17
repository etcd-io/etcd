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

package e2e

import (
	"os"

	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/expect"
)

var (
	initBinPath    func(string) binPath
	additionalArgs func() ([]string, error)
)

func SpawnCmd(lg *zap.Logger, name string, args []string, envVars map[string]string) (*expect.ExpectProcess, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	newArgs, err := additionalArgs()
	if err != nil {
		return nil, err
	}
	env := mergeEnvVariables(envVars)
	lg.Info("spawning process",
		zap.Strings("args", args),
		zap.String("working-dir", wd),
		zap.String("name", name),
		zap.Strings("environment-variables", env))
	return expect.NewExpectWithEnv(args[0], append(args[1:], newArgs...), env, name)
}
