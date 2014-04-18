package test

import (
	"os"
	"testing"
	"time"
)

func TestJoinThroughFollower(t *testing.T) {
	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	_, etcds, err := CreateCluster(2, procAttr, false)
	if err != nil {
		t.Fatal("cannot create cluster")
	}
	defer DestroyCluster(etcds)

	time.Sleep(time.Second)

	newEtcd, err := os.StartProcess(EtcdBinPath, []string{"etcd", "-data-dir=/tmp/node3", "-name=node3", "-addr=127.0.0.1:4003", "-peer-addr=127.0.0.1:7003", "-peers=127.0.0.1:7002", "-f"}, procAttr)
	if err != nil {
		t.Fatal("failed starting node3")
	}
	defer func() {
		newEtcd.Kill()
		newEtcd.Release()
	}()

	time.Sleep(time.Second)

	leader, err := getLeader("http://127.0.0.1:4003")
	if err != nil {
		t.Fatal("failed getting leader from node3:", err)
	}
	if leader != "http://127.0.0.1:7001" {
		t.Fatal("expect=http://127.0.0.1:7001 got=", leader)
	}
}
