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

package client_test

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.etcd.io/etcd/v3/integration"
	"go.etcd.io/etcd/v3/pkg/testutil"
	"go.etcd.io/etcd/v3/pkg/transport"
)

var exampleEndpoints []string
var exampleTransport *http.Transport

// TestMain sets up an etcd cluster if running the examples.
func TestMain(m *testing.M) {
	useCluster, hasRunArg := false, false // default to running only Test*
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.run=") {
			exp := strings.Split(arg, "=")[1]
			match, err := regexp.MatchString(exp, "Example")
			useCluster = (err == nil && match) || strings.Contains(exp, "Example")
			hasRunArg = true
			break
		}
	}
	if !hasRunArg {
		// force only running Test* if no args given to avoid leak false
		// positives from having a long-running cluster for the examples.
		os.Args = append(os.Args, "-test.run=Test")
	}

	var v int
	if useCluster {
		tr, trerr := transport.NewTransport(transport.TLSInfo{}, time.Second)
		if trerr != nil {
			fmt.Fprintf(os.Stderr, "%v", trerr)
			os.Exit(1)
		}
		cfg := integration.ClusterConfig{Size: 1}
		clus := integration.NewClusterV3(nil, &cfg)
		exampleEndpoints = []string{clus.Members[0].URL()}
		exampleTransport = tr
		v = m.Run()
		clus.Terminate(nil)
		if err := testutil.CheckAfterTest(time.Second); err != nil {
			fmt.Fprintf(os.Stderr, "%v", err)
			os.Exit(1)
		}
	} else {
		v = m.Run()
	}

	if v == 0 && testutil.CheckLeakedGoroutine() {
		os.Exit(1)
	}
	os.Exit(v)
}
