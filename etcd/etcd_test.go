/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package etcd

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/etcd/config"
)

func TestRunStop(t *testing.T) {
	path, _ := ioutil.TempDir("", "etcd-")
	defer os.RemoveAll(path)

	config := config.New()
	config.Name = "ETCDTEST"
	config.DataDir = path
	config.Addr = "localhost:0"
	config.Peer.Addr = "localhost:0"

	etcd := New(config)
	go etcd.Run()
	<-etcd.ReadyNotify()
	etcd.Stop()
}
