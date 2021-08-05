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

// Package main is a simple wrapper of the real etcd entrypoint package
// (located at go.etcd.io/etcd/etcdmain) to ensure that etcd is still
// "go getable"; e.g. `go get go.etcd.io/etcd` works as expected and
// builds a binary in $GOBIN/etcd
//
// This package should NOT be extended or modified in any way; to modify the
// etcd binary, work in the `go.etcd.io/etcd/etcdmain` package.
//

// Tips: 这个main函数是包装 etcdmain ，这样在用 go get go.etcd.io/etcd 命令时，会自动编译etcd二进制文件
package main

import (
	"os"

	"go.etcd.io/etcd/server/v3/etcdmain"
)
/*
./etcd --name infra1 --listen-client-urls http://127.0.0.1:2379 --advertise-client-urls http://127.0.0.1:2379
--listen-peer-urls http://127.0.0.1:12380 --initial-advertise-peer-urls http://127.0.0.1:12380 --initial-cluster-token etcd-cluster-1
--initial-cluster 'infra1=http://127.0.0.1:12380,infra2=http://127.0.0.1:22380,infra3=http://127.0.0.1:32380' --initial-cluster-state new
--enable-pprof --logger=zap --log-outputs=stderr
*/
func main() {
	etcdmain.Main(os.Args)
}
