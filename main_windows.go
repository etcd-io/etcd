// Copyright 2015 CoreOS, Inc.
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

// Package main is a simple wrapper of the real etcd entrypoint package
// (located at github.com/coreos/etcd/etcdmain) to ensure that etcd is still
// "go getable"; e.g. `go get github.com/coreos/etcd` works as expected and
// builds a binary in $GOBIN/etcd
//
// This package should NOT be extended or modified in any way; to modify the
// etcd binary, work in the `github.com/coreos/etcd/etcdmain` package.
//

// Run as a windows service:
//	sc create etcd binPath= "C:\etcd\etcd.exe -data-dir=C:\etcd\etcd.service.data"
//	net start etcd
//	net stop etcd
//	sc delete etcd

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coreos/etcd/etcdmain"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

func main() {
	isIntSess, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("etcd: failed to determine if we are running in an interactive session: %v", err)
	}
	if !isIntSess {
		svcName := filepath.Base(os.Args[0])
		if strings.HasSuffix(strings.ToLower(svcName), ".exe") {
			svcName = svcName[:len(svcName)-len(".exe")]
		}
		runAsService(svcName)
		return
	}

	etcdmain.Main()
}

func runAsService(name string) {
	elog, err := eventlog.Open(name)
	if err != nil {
		return
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("etcd: starting %s service", name))
	if err = svc.Run(name, &etcdService{elog: elog}); err != nil {
		elog.Error(1, fmt.Sprintf("etcd: %s service failed: %v", name, err))
		return
	}
	elog.Info(1, fmt.Sprintf("etcd: %s service stopped", name))
}

type etcdService struct {
	elog debug.Log
}

func (m *etcdService) StartServer() {
	exepath, err := exePath()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.Chdir(filepath.Dir(exepath)); err != nil {
		log.Fatal(err)
	}
	etcdmain.Main()
}

func (m *etcdService) StopServer() {
	//
}

func (m *etcdService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	go m.StartServer()
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			default:
				m.elog.Error(1, fmt.Sprintf("etcd: unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	m.StopServer()
	return
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("etcd: %s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"

		var fi os.FileInfo
		fi, err = os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("etcd: %s is directory", p)
		}
	}
	return "", err
}
