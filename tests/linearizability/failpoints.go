// Copyright 2022 The etcd Authors
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

package linearizability

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

var (
	KillFailpoint       Failpoint = killFailpoint{}
	RaftBeforeSavePanic Failpoint = goFailpoint{"etcdserver/raftBeforeSave", "panic"}
)

type Failpoint interface {
	Trigger(ctx context.Context, clus *e2e.EtcdProcessCluster) error
}

type killFailpoint struct{}

func (f killFailpoint) Trigger(ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	err := member.Kill()
	if err != nil {
		return err
	}
	err = member.Wait()
	if err != nil {
		return err
	}
	err = member.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

type goFailpoint struct {
	failpoint string
	payload   string
}

func (f goFailpoint) Trigger(ctx context.Context, clus *e2e.EtcdProcessCluster) error {
	member := clus.Procs[rand.Int()%len(clus.Procs)]
	address := fmt.Sprintf("127.0.0.1:%d", member.Config().GoFailPort)
	err := triggerGoFailpoint(address, f.failpoint, f.payload)
	if err != nil {
		return fmt.Errorf("failed to trigger failpoint %q, err: %v", f.failpoint, err)
	}
	err = clus.Procs[0].Wait()
	if err != nil {
		return err
	}
	err = clus.Procs[0].Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func triggerGoFailpoint(host, failpoint, payload string) error {
	failpointUrl := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   failpoint,
	}
	r, err := http.NewRequest("PUT", failpointUrl.String(), bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

var httpClient = http.Client{
	Timeout: 10 * time.Millisecond,
}
