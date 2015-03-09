package main

import (
	"fmt"
	"os"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/juju/loggo"
)

var logger = loggo.GetLogger("main")
var rootLogger = loggo.GetLogger("")

func main() {
	args := os.Args
	if len(args) > 1 {
		loggo.ConfigureLoggers(args[1])
	} else {
		fmt.Println("Add a parameter to configure the logging:")
		fmt.Println("E.g. \"<root>=INFO;first=TRACE\"")
	}
	fmt.Println("\nCurrent logging levels:")
	fmt.Println(loggo.LoggerInfo())
	fmt.Println("")

	rootLogger.Infof("Start of test.")

	FirstCritical("first critical")
	FirstError("first error")
	FirstWarning("first warning")
	FirstInfo("first info")
	FirstTrace("first trace")

	SecondCritical("first critical")
	SecondError("first error")
	SecondWarning("first warning")
	SecondInfo("first info")
	SecondTrace("first trace")

}
