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

package main

import (
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/coreos/etcd/etcdctlv3/command"
	"github.com/coreos/etcd/version"
)

func main() {
	app := cli.NewApp()
	app.Name = "etcdctlv3"
	app.Version = version.Version
	app.Usage = "A simple command line client for etcd3."
	app.Commands = []cli.Command{
		command.NewRangeCommand(),
		command.NewPutCommand(),
		command.NewDeleteRangeCommand(),
	}

	app.Run(os.Args)
}
