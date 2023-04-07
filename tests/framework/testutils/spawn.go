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

package testutils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/expect"
)

func SpawnCmd(args []string, envVars map[string]string) (*expect.ExpectProcess, error) {
	return SpawnNamedCmd(strings.Join(args, "_"), args, envVars)
}

func SpawnNamedCmd(processName string, args []string, envVars map[string]string) (*expect.ExpectProcess, error) {
	return SpawnCmdWithLogger(zap.NewNop(), args, envVars, processName)
}

func SpawnCmdWithLogger(lg *zap.Logger, args []string, envVars map[string]string, name string) (*expect.ExpectProcess, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	env := mergeEnvVariables(envVars)
	lg.Info("spawning process",
		zap.Strings("args", args),
		zap.String("working-dir", wd),
		zap.String("name", name),
		zap.Strings("environment-variables", env))
	return expect.NewExpectWithEnv(args[0], args[1:], env, name)
}

func WaitReadyExpectProc(ctx context.Context, exproc *expect.ExpectProcess, readyStrs []string) error {
	matchSet := func(l string) bool {
		for _, s := range readyStrs {
			if strings.Contains(l, s) {
				return true
			}
		}
		return false
	}
	_, err := exproc.ExpectFunc(ctx, matchSet)
	return err
}

func SpawnWithExpect(args []string, expected string) error {
	return SpawnWithExpects(args, nil, []string{expected}...)
}

func SpawnWithExpectWithEnv(args []string, envVars map[string]string, expected string) error {
	return SpawnWithExpects(args, envVars, []string{expected}...)
}

func SpawnWithExpects(args []string, envVars map[string]string, xs ...string) error {
	return SpawnWithExpectsContext(context.TODO(), args, envVars, xs...)
}

func SpawnWithExpectsContext(ctx context.Context, args []string, envVars map[string]string, xs ...string) error {
	_, err := SpawnWithExpectLines(ctx, args, envVars, xs...)
	return err
}

func SpawnWithExpectLines(ctx context.Context, args []string, envVars map[string]string, xs ...string) ([]string, error) {
	proc, err := SpawnCmd(args, envVars)
	if err != nil {
		return nil, err
	}
	defer proc.Close()
	// process until either stdout or stderr contains
	// the expected string
	var (
		lines []string
	)
	for _, txt := range xs {
		l, lerr := proc.ExpectWithContext(ctx, txt)
		if lerr != nil {
			proc.Close()
			return nil, fmt.Errorf("%v %v (expected %q, got %q). Try EXPECT_DEBUG=TRUE", args, lerr, txt, lines)
		}
		lines = append(lines, l)
	}
	perr := proc.Close()
	if perr != nil {
		return lines, fmt.Errorf("err: %w, with output lines %v", perr, proc.Lines())
	}

	l := proc.LineCount()
	if len(xs) == 0 && l != 0 { // expect no output
		return nil, fmt.Errorf("unexpected output from %v (got lines %q, line count %d). Try EXPECT_DEBUG=TRUE", args, lines, l)
	}
	return lines, nil
}

func RunUtilCompletion(args []string, envVars map[string]string) ([]string, error) {
	proc, err := SpawnCmd(args, envVars)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn command %v with error: %w", args, err)
	}

	proc.Wait()
	err = proc.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close command %v with error: %w", args, err)
	}

	return proc.Lines(), nil
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
		// TODO: Remove PATH when we stop using system binaries (`awk`, `echo`)
		if !strings.HasPrefix(p[0], "ETCD_") && !strings.HasPrefix(p[0], "ETCDCTL_") && !strings.HasPrefix(p[0], "EXPECT_") && p[0] != "PATH" {
			continue
		}
		if _, ok := envVars[p[0]]; !ok {
			env = append(env, fmt.Sprintf("%s=%s", p[0], p[1]))
		}
	}

	return env
}
