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

package integration

import (
	"github.com/coreos/etcd/pkg/types"
	"net"
	"os/exec"
	"testing"
)

func TestUpgradeMember(t *testing.T) {
	defer afterTest(t)
	m := mustNewMember(t, "integration046")
	newPeerListeners := make([]net.Listener, 0)
	newPeerListeners = append(newPeerListeners, newListenerWithAddr(t, "127.0.0.1:59892"))
	m.PeerListeners = newPeerListeners
	urls, err := types.NewURLs([]string{"http://127.0.0.1:59892"})
	if err != nil {
		t.Fatal(err)
	}
	m.PeerURLs = urls
	m.NewCluster = true
	c := &cluster{}
	c.Members = []*member{m}
	fillClusterForMembers(c.Members, "etcd-cluster")
	cmd := exec.Command("cp", "-r", "testdata/integration046_data/conf", "testdata/integration046_data/log", "testdata/integration046_data/snapshot", m.DataDir)
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}

	c.Launch(t)
	defer c.Terminate(t)
	clusterMustProgress(t, c.Members)
}
