// Copyright 2015 The etcd Authors
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

package etcdmain

import (
	"fmt"
	"os"
	"strings"

	"github.com/coreos/go-systemd/daemon"
	systemdutil "github.com/coreos/go-systemd/util"
	"github.com/kardianos/service"
	"go.uber.org/zap"
)

func checkAndStart() {
	checkSupportArch()

	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if covArgs := os.Getenv("ETCDCOV_ARGS"); len(covArgs) > 0 {
			args := strings.Split(os.Getenv("ETCDCOV_ARGS"), "\xe7\xcd")[1:]
			rootCmd.SetArgs(args)
			cmd = "grpc-proxy"
		}
		switch cmd {
		case "gateway", "grpc-proxy":
			if err := rootCmd.Execute(); err != nil {
				fmt.Fprint(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
	}

	startEtcdOrProxyV2()
}

func notifySystemd(lg *zap.Logger) {
	if !systemdutil.IsRunningSystemd() {
		return
	}

	if lg != nil {
		lg.Info("host was booted with systemd, sends READY=1 message to init daemon")
	}
	sent, err := daemon.SdNotify(false, "READY=1")
	if err != nil {
		if lg != nil {
			lg.Error("failed to notify systemd for readiness", zap.Error(err))
		} else {
			plog.Errorf("failed to notify systemd for readiness: %v", err)
		}
	}

	if !sent {
		if lg != nil {
			lg.Warn("forgot to set Type=notify in systemd service file?")
		} else {
			plog.Errorf("forgot to set Type=notify in systemd service file?")
		}
	}
}

var logger service.Logger

func Main() {
	svcConfig := &service.Config{
		// Required name of the service. No spaces suggested.
		Name: "etcd daemon",
		// Long description of service.
		Description: "graceful running etcd",
	}

	prg := &program{}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		plog.Errorf("failed to init daemon service: %v", err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		plog.Errorf("failed to init daemon logger: %v", err)
	}

	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

type program struct {
	Stderr, Stdout string //TODO use logger or lg or plog?
}

func (p *program) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	go p.run()
	return nil
}

func (p *program) run() {
	checkAndStart()
}

func (p *program) Stop(s service.Service) error {
	// Stop should not block. Return with a few seconds or do something
	return s.Stop()
}
