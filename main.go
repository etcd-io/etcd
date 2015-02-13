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

// Package main is a simple wrapper of the real etcd entrypoint package
// (located at github.com/coreos/etcd/etcdmain) to ensure that etcd is still
// "go getable"; e.g. `go get github.com/coreos/etcd` works as expected and
// builds a binary in $GOBIN/etcd
//
// This package should NOT be extended or modified in any way; to modify the
// etcd binary, work in the `github.com/coreos/etcd/etcdmain` package.
//

package main

import (
	"log"
	"os"
	"strconv"

	"github.com/coreos/etcd/etcdmain"
	"github.com/coreos/etcd/migrate/starter"
	"github.com/coreos/etcd/pkg/coreos"
)

func main() {
	if str := os.Getenv("ETCD_ALLOW_LEGACY_MODE"); str != "" {
		v, err := strconv.ParseBool(str)
		if err != nil {
			log.Fatalf("failed to parse ETCD_ALLOW_LEGACY_MODE=%s as bool", str)
		}
		if v {
			starter.StartDesiredVersion(os.Args[1:])
		}
	} else if coreos.IsCoreOS() {
		starter.StartDesiredVersion(os.Args[1:])
	}
	etcdmain.Main()
}
