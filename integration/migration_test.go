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
