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

package command

import (
	"github.com/urfave/cli"
	"go.etcd.io/etcd/etcdutl/v3/etcdutl"
)

const (
	description = "Performs an offline backup of etcd directory.\n\n" +
		"Moved to `./etcdutl backup` and going to be decomissioned in v3.5\n\n" +
		"The recommended (online) backup command is: `./etcdctl snapshot save ...`.\n\n"
)

func NewBackupCommand() cli.Command {
	return cli.Command{
		Name:        "backup",
		Usage:       "--data-dir=... --backup-dir={output}",
		UsageText:   "[deprecated] offline backup an etcd directory.",
		Description: description,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "data-dir", Value: "", Usage: "Path to the etcd data dir"},
			cli.StringFlag{Name: "wal-dir", Value: "", Usage: "Path to the etcd wal dir"},
			cli.StringFlag{Name: "backup-dir", Value: "", Usage: "Path to the backup dir"},
			cli.StringFlag{Name: "backup-wal-dir", Value: "", Usage: "Path to the backup wal dir"},
			cli.BoolFlag{Name: "with-v3", Usage: "Backup v3 backend data"},
		},
		Action: handleBackup,
	}
}

func handleBackup(c *cli.Context) error {
	etcdutl.HandleBackup(
		c.Bool("with-v3"),
		c.String("data-dir"),
		c.String("backup-dir"),
		c.String("wal-dir"),
		c.String("backup-wal-dir"),
	)
	return nil
}
