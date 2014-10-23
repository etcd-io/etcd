package main

import (
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/codegangsta/cli"

	"github.com/coreos/etcd/etcdctl/command"
	"github.com/coreos/etcd/version"
)

func main() {
	app := cli.NewApp()
	app.Name = "etcdctl"
	app.Version = version.Version
	app.Usage = "A simple command line client for etcd."
	app.Flags = []cli.Flag{
		cli.BoolFlag{Name: "debug", Usage: "output cURL commands which can be used to reproduce the request"},
		cli.BoolFlag{Name: "no-sync", Usage: "don't synchronize cluster information before sending request"},
		cli.StringFlag{Name: "output, o", Value: "simple", Usage: "output response in the given format (`simple` or `json`)"},
		cli.StringFlag{Name: "peers, C", Value: "", Usage: "a comma-delimited list of machine addresses in the cluster (default: \"127.0.0.1:4001\")"},
	}
	app.Commands = []cli.Command{
		command.NewMakeCommand(),
		command.NewMakeDirCommand(),
		command.NewRemoveCommand(),
		command.NewRemoveDirCommand(),
		command.NewGetCommand(),
		command.NewLsCommand(),
		command.NewSetCommand(),
		command.NewSetDirCommand(),
		command.NewUpdateCommand(),
		command.NewUpdateDirCommand(),
		command.NewWatchCommand(),
		command.NewExecWatchCommand(),
	}
	app.Run(os.Args)
}
