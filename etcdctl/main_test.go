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
	"strings"
	"testing"
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

// Empty test to avoid no-tests warning.
func TestEmpty(t *testing.T) {}

/**
 * The purpose of this "test" is to run etcdctl with code-coverage
 * collection turned on. The technique is documented here:
 *
 * https://www.cyphar.com/blog/post/20170412-golang-integration-coverage
 */
func TestMain(m *testing.M) {
	// don't launch etcdctl when invoked via go test
	if strings.HasSuffix(os.Args[0], "etcdctl.test") {
		return
	}

	testArgs, appArgs := SplitTestArgs(os.Args)

	os.Args = appArgs

	err := mainWithError()
	if err != nil {
		log.Fatalf("etcdctl failed with: %v", err)
	}

	// This will generate coverage files:
	os.Args = testArgs
	m.Run()
}
