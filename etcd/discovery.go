/*
Copyright 2014 CoreOS Inc.

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
	"errors"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"time"

	etcdErr "github.com/coreos/etcd/error"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
)

const (
	stateKey     = "_state"
	startedState = "started"
	defaultTTL   = 604800 // One week TTL
)

type discoverer struct {
	client *etcd.Client
	name   string
	addr   string
	prefix string
}

func newDiscoverer(u *url.URL, name, raftPubAddr string) *discoverer {
	d := &discoverer{name: name, addr: raftPubAddr}

	// prefix is prepended to all keys for this discovery
	d.prefix = strings.TrimPrefix(u.Path, "/v2/keys/")

	// keep the old path in case we need to set the KeyPrefix below
	oldPath := u.Path
	u.Path = ""

	// Connect to a scheme://host not a full URL with path
	log.Printf("Discovery via %s using prefix %s.\n", u.String(), d.prefix)
	d.client = etcd.NewClient([]string{u.String()})

	if !strings.HasPrefix(oldPath, "/v2/keys") {
		d.client.SetKeyPrefix("")
	}
	return d
}

func (d *discoverer) discover() ([]string, error) {
	if _, err := d.client.Set(path.Join(d.prefix, d.name), d.addr, defaultTTL); err != nil {
		return nil, fmt.Errorf("discovery service error: %v", err)
	}

	// Attempt to take the leadership role, if there is no error we are it!
	resp, err := d.client.Create(path.Join(d.prefix, stateKey), startedState, 0)
	// Bail out on unexpected errors
	if err != nil {
		if clientErr, ok := err.(*etcd.EtcdError); !ok || clientErr.ErrorCode != etcdErr.EcodeNodeExist {
			return nil, fmt.Errorf("discovery service error: %v", err)
		}
	}

	// If we got a response then the CAS was successful, we are leader
	if resp != nil && resp.Node.Value == startedState {
		// We are the leader, we have no peers
		log.Println("Discovery _state was empty, so this machine is the initial leader.")
		return nil, nil
	}

	// Fall through to finding the other discovery peers
	return d.findPeers()
}

func (d *discoverer) findPeers() (peers []string, err error) {
	resp, err := d.client.Get(path.Join(d.prefix), false, true)
	if err != nil {
		return nil, fmt.Errorf("discovery service error: %v", err)
	}

	node := resp.Node

	if node == nil {
		return nil, fmt.Errorf("%s key doesn't exist.", d.prefix)
	}

	for _, n := range node.Nodes {
		// Skip our own entry in the list, there is no point
		if strings.HasSuffix(n.Key, "/"+d.name) {
			continue
		}
		peers = append(peers, n.Value)
	}

	if len(peers) == 0 {
		return nil, errors.New("Discovery found an initialized cluster but no reachable peers are registered.")
	}

	log.Printf("Discovery found peers %v\n", peers)
	return
}

func (d *discoverer) heartbeat(stopc <-chan struct{}) {
	// In case of errors we should attempt to heartbeat fairly frequently
	heartbeatInterval := defaultTTL / 8
	ticker := time.NewTicker(time.Second * time.Duration(heartbeatInterval))
	defer ticker.Stop()
	for {
		if _, err := d.client.Set(path.Join(d.prefix, d.name), d.addr, defaultTTL); err != nil {
			log.Println("Discovery heartbeat failed: %v", err)
		}

		select {
		case <-ticker.C:
		case <-stopc:
			return
		}
	}
}
