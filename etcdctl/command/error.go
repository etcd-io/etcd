package command

import (
	"fmt"
	"os"
)

const (
	SUCCESS = iota
	MalformedEtcdctlArguments
	FailedToConnectToHost
	FailedToAuth
	ErrorFromEtcd
)

func handleError(code int, err error) {
	fmt.Fprintln(os.Stderr, "Error: ", err)
	os.Exit(code)
}
