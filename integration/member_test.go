package integration

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestRestartMember(t *testing.T) {
	defer afterTest(t)
	c := NewCluster(t, 3)
	c.Launch(t)
	defer c.Terminate(t)

	for i := 0; i < 3; i++ {
		c.Members[i].Stop(t)
		err := c.Members[i].Restart(t)
		if err != nil {
			t.Fatal(err)
		}
	}
	clusterMustProgress(t, c)
}

func TestLaunchDuplicateMemberShouldFail(t *testing.T) {
	size := 3
	c := NewCluster(t, size)
	m := c.Members[0].Clone()
	var err error
	m.DataDir, err = ioutil.TempDir(os.TempDir(), "etcd")
	if err != nil {
		t.Fatal(err)
	}
	c.Launch(t)
	defer c.Terminate(t)

	if err := m.Launch(); err == nil {
		t.Errorf("unexpect successful launch")
	}
}
