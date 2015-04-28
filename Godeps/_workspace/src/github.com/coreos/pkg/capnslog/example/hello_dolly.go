package main

import (
	oldlog "log"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/pkg/capnslog"
)

var log = capnslog.NewPackageLogger("github.com/coreos/pkg/capnslog/cmd", "main")
var dlog = capnslog.NewPackageLogger("github.com/coreos/pkg/capnslog/cmd", "dolly")

func main() {
	rl := capnslog.MustRepoLogger("github.com/coreos/pkg/capnslog/cmd")
	capnslog.SetFormatter(capnslog.NewStringFormatter(os.Stderr))

	// We can parse the log level configs from the command line
	if len(os.Args) > 1 {
		cfg, err := rl.ParseLogLevelConfig(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		rl.SetLogLevel(cfg)
		log.Infof("Setting output to %s", os.Args[1])
	}

	// Send some messages at different levels to the different packages
	dlog.Infof("Hello Dolly")
	dlog.Warningf("Well hello, Dolly")
	log.Errorf("It's so nice to have you back where you belong")
	dlog.Debugf("You're looking swell, Dolly")
	dlog.Tracef("I can tell, Dolly")

	// We also have control over the built-in "log" package.
	capnslog.SetGlobalLogLevel(capnslog.INFO)
	oldlog.Println("You're still glowin', you're still crowin', you're still lookin' strong")
	log.Fatalf("Dolly'll never go away again")
}
