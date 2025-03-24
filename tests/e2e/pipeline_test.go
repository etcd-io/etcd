package e2e

import (
	"context"
	"testing"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestPipelineBufferSize(t *testing.T) {
	ctx := context.Background()

	// First cluster with buffer size 128
	epc1, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithPipelineBufferSize(128),
		e2e.WithBasePort(20000),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster: %v", err)
	}
	defer epc1.Close()

	// Wait for the cluster to be ready
	time.Sleep(time.Second)

	// Verify the first cluster has the correct buffer size
	for _, proc := range epc1.Procs {
		args := proc.Config().Args
		found := false
		for _, arg := range args {
			if arg == "--pipeline-buffer-size=128" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("member %s does not have pipeline-buffer-size=128 in args %v", proc.Config().Name, args)
		}
	}

	// Second cluster with buffer size 256
	epc2, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(3),
		e2e.WithPipelineBufferSize(256),
		e2e.WithBasePort(21000),
	)
	if err != nil {
		t.Fatalf("could not start etcd process cluster: %v", err)
	}
	defer epc2.Close()

	// Wait for the cluster to be ready
	time.Sleep(time.Second)

	// Verify the second cluster has the correct buffer size
	for _, proc := range epc2.Procs {
		args := proc.Config().Args
		found := false
		for _, arg := range args {
			if arg == "--pipeline-buffer-size=256" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("member %s does not have pipeline-buffer-size=256 in args %v", proc.Config().Name, args)
		}
	}
}
