// Copyright 2016 The etcd Authors
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

// These tests depend on certificate-based authentication that is NOT supported
// by gRPC proxy.
//go:build !cluster_proxy
// +build !cluster_proxy

package e2e

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCtlV3AuthCertCN(t *testing.T) {
	testCtl(t, authTestCertCN, withCfg(*newConfigClientTLSCertAuth()))
}
func TestCtlV3AuthCertCNAndUsername(t *testing.T) {
	testCtl(t, authTestCertCNAndUsername, withCfg(*newConfigClientTLSCertAuth()))
}
func TestCtlV3AuthCertCNAndUsernameNoPassword(t *testing.T) {
	testCtl(t, authTestCertCNAndUsernameNoPassword, withCfg(*newConfigClientTLSCertAuth()))
}

func TestCtlV3AuthCertCNWithWithConcurrentOperation(t *testing.T) {
	BeforeTest(t)

	// apply the certificate which has `root` CommonName,
	// and reset the setting when the test case finishes.
	// TODO(ahrtr): enhance the e2e test framework to support
	// certificates with CommonName.
	t.Log("Apply certificate with root CommonName")
	resetCert := applyTLSWithRootCommonName()
	defer resetCert()

	t.Log("Create an etcd cluster")
	cx := getDefaultCtlCtx(t)
	cx.cfg = etcdProcessClusterConfig{
		clusterSize:           1,
		clientTLS:             clientTLS,
		clientCertAuthEnabled: true,
		initialToken:          "new",
	}

	epc, err := newEtcdProcessCluster(t, &cx.cfg)
	if err != nil {
		t.Fatalf("Failed to start etcd cluster: %v", err)
	}
	cx.epc = epc
	cx.dataDir = epc.procs[0].Config().dataDirPath

	defer func() {
		if err := epc.Close(); err != nil {
			t.Fatalf("could not close test cluster (%v)", err)
		}
	}()

	t.Log("Enable auth")
	authEnableTest(cx)

	// Create two goroutines, one goroutine keeps creating & deleting users,
	// and the other goroutine keeps writing & deleting K/V entries.
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	donec := make(chan struct{})

	// Create the first goroutine to create & delete users
	t.Log("Create the first goroutine to create & delete users")
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			user := fmt.Sprintf("testuser-%d", i)
			pass := fmt.Sprintf("testpass-%d", i)

			if err := ctlV3User(cx, []string{"add", user, "--interactive=false"}, fmt.Sprintf("User %s created", user), []string{pass}); err != nil {
				errs <- fmt.Errorf("failed to create user %q: %w", user, err)
				break
			}

			err := ctlV3User(cx, []string{"delete", user}, fmt.Sprintf("User %s deleted", user), []string{})
			if err != nil {
				errs <- fmt.Errorf("failed to delete user %q: %w", user, err)
				break
			}
		}
		t.Log("The first goroutine finished")
	}()

	// Create the second goroutine to write & delete K/V entries
	t.Log("Create the second goroutine to write & delete K/V entries")
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key-%d", i)
			value := fmt.Sprintf("value-%d", i)

			if err := ctlV3Put(cx, key, value, ""); err != nil {
				errs <- fmt.Errorf("failed to put key %q: %w", key, err)
				break
			}

			if err := ctlV3Del(cx, []string{key}, 1); err != nil {
				errs <- fmt.Errorf("failed to delete key %q: %w", key, err)
				break
			}
		}
		t.Log("The second goroutine finished")
	}()

	t.Log("Waiting for the two goroutines to complete")
	go func() {
		wg.Wait()
		close(donec)
	}()

	t.Log("Waiting for test result")
	select {
	case err := <-errs:
		t.Fatalf("Unexpected error: %v", err)
	case <-donec:
		t.Log("All done!")
	case <-time.After(60 * time.Second):
		t.Fatal("Test case timeout after 60 seconds")
	}
}
