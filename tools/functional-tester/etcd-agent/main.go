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

package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/coreos/pkg/capnslog"
)

var plog = capnslog.NewPackageLogger("github.com/coreos/etcd", "etcd-agent")

func main() {
	etcdPath := flag.String("etcd-path", filepath.Join(os.Getenv("GOPATH"), "bin/etcd"), "the path to etcd binary")
	etcdLogDir := flag.String("etcd-log-dir", "etcd-log", "directory to store etcd logs, data directories, failure archive")
	port := flag.String("port", ":9027", "port to serve agent server")
	useRoot := flag.Bool("use-root", true, "use root permissions")
	failpointAddr := flag.String("failpoint-addr", ":2381", "interface for gofail's HTTP server")
	logFilePath := flag.String("log-file", "", "log file path (leave empty to log to stderr)")
	flag.Parse()

	if *logFilePath != "" {
		f, err := os.OpenFile(*logFilePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0600)
		if err != nil {
			plog.Fatal(err)
		}
		defer f.Close()
		capnslog.SetFormatter(capnslog.NewDefaultFormatter(f))
	}

	cfg := AgentConfig{
		EtcdPath:      *etcdPath,
		LogDir:        *etcdLogDir,
		FailpointAddr: *failpointAddr,
		UseRoot:       *useRoot,
	}

	if *useRoot && os.Getuid() != 0 {
		plog.Println("got --use-root=true but not root user")
		os.Exit(1)
	}
	if !*useRoot {
		plog.Println("root permissions disabled, agent will not modify network")
	}

	a, err := newAgent(cfg)
	if err != nil {
		plog.Fatal(err)
	}
	a.serveRPC(*port)

	var done chan struct{}
	<-done
}
