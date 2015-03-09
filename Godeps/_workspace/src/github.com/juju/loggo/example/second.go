package main

import (
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/juju/loggo"
)

var second = loggo.GetLogger("second")

func SecondCritical(message string) {
	second.Criticalf(message)
}

func SecondError(message string) {
	second.Errorf(message)
}

func SecondWarning(message string) {
	second.Warningf(message)
}

func SecondInfo(message string) {
	second.Infof(message)
}

func SecondTrace(message string) {
	second.Tracef(message)
}
