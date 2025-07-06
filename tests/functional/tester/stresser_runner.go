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

package tester

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"syscall"

	"go.etcd.io/etcd/tests/v3/functional/rpcpb"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type runnerStresser struct {
	stype              rpcpb.StresserType
	etcdClientEndpoint string
	lg                 *zap.Logger

	cmd     *exec.Cmd
	cmdStr  string
	args    []string
	rl      *rate.Limiter
	reqRate int

	errc  chan error
	donec chan struct{}
}

func newRunnerStresser(
	stype rpcpb.StresserType,
	ep string,
	lg *zap.Logger,
	cmdStr string,
	args []string,
	rl *rate.Limiter,
	reqRate int,
) *runnerStresser {
	rl.SetLimit(rl.Limit() - rate.Limit(reqRate))
	return &runnerStresser{
		stype:              stype,
		etcdClientEndpoint: ep,
		lg:                 lg,
		cmdStr:             cmdStr,
		args:               args,
		rl:                 rl,
		reqRate:            reqRate,
		errc:               make(chan error, 1),
		donec:              make(chan struct{}),
	}
}

func (rs *runnerStresser) setupOnce() (err error) {
	if rs.cmd != nil {
		return nil
	}

	rs.cmd = exec.Command(rs.cmdStr, rs.args...)
	stderr, err := rs.cmd.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		defer close(rs.donec)
		out, err := ioutil.ReadAll(stderr)
		if err != nil {
			rs.errc <- err
		} else {
			rs.errc <- fmt.Errorf("(%v %v) stderr %v", rs.cmdStr, rs.args, string(out))
		}
	}()

	return rs.cmd.Start()
}

func (rs *runnerStresser) Stress() (err error) {
	rs.lg.Info(
		"stress START",
		zap.String("stress-type", rs.stype.String()),
	)
	if err = rs.setupOnce(); err != nil {
		return err
	}
	return syscall.Kill(rs.cmd.Process.Pid, syscall.SIGCONT)
}

func (rs *runnerStresser) Pause() map[string]int {
	rs.lg.Info(
		"stress STOP",
		zap.String("stress-type", rs.stype.String()),
	)
	syscall.Kill(rs.cmd.Process.Pid, syscall.SIGSTOP)
	return nil
}

func (rs *runnerStresser) Close() map[string]int {
	syscall.Kill(rs.cmd.Process.Pid, syscall.SIGINT)
	rs.cmd.Wait()
	<-rs.donec
	rs.rl.SetLimit(rs.rl.Limit() + rate.Limit(rs.reqRate))
	return nil
}

func (rs *runnerStresser) ModifiedKeys() int64 {
	return 1
}
