// Copyright 2017 The etcd Authors
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
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	"go.etcd.io/etcd/server/v3/etcdmain"
)

func SplitTestArgs(args []string) (testArgs, appArgs []string) {
	for i, arg := range os.Args {
		switch {
		case strings.HasPrefix(arg, "-test."):
			testArgs = append(testArgs, arg)
		case i == 0:
			appArgs = append(appArgs, arg)
			testArgs = append(testArgs, arg)
		default:
			appArgs = append(appArgs, arg)
		}
	}
	return
}

func TestEmpty(t *testing.T) {}

/**
 * The purpose of this "test" is to run etcd server with code-coverage
 * collection turned on. The technique is documented here:
 *
 * https://www.cyphar.com/blog/post/20170412-golang-integration-coverage
 */
func TestMain(m *testing.M) {
	// don't launch etcd server when invoked via go test
	if strings.HasSuffix(os.Args[0], ".test") {
		log.Printf("skip launching etcd server when invoked via go test")
		return
	}

	testArgs, appArgs := SplitTestArgs(os.Args)

	notifier := make(chan os.Signal, 1)
	signal.Notify(notifier, syscall.SIGINT, syscall.SIGTERM)
	go etcdmain.Main(appArgs)
	<-notifier

	// This will generate coverage files:
	os.Args = testArgs
	m.Run()
}
