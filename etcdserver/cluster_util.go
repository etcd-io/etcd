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

package etcdserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/coreos/etcd/pkg/types"
)

// isMemberBootstrapped tries to check if the given member has been bootstrapped
// in the given cluster.
func isMemberBootstrapped(cl *Cluster, member string, tr *http.Transport) bool {
	us := getRemotePeerURLs(cl, member)
	rcl, err := getClusterFromPeers(us, false, tr)
	if err != nil {
		return false
	}
	id := cl.MemberByName(member).ID
	m := rcl.Member(id)
	if m == nil {
		return false
	}
	if len(m.ClientURLs) > 0 {
		return true
	}
	return false
}

// GetClusterFromPeers takes a set of URLs representing etcd peers, and
// attempts to construct a Cluster by accessing the members endpoint on one of
// these URLs. The first URL to provide a response is used. If no URLs provide
// a response, or a Cluster cannot be successfully created from a received
// response, an error is returned.
func GetClusterFromPeers(urls []string, tr *http.Transport) (*Cluster, error) {
	return getClusterFromPeers(urls, true, tr)
}

// If logerr is true, it prints out more error messages.
func getClusterFromPeers(urls []string, logerr bool, tr *http.Transport) (*Cluster, error) {
	cc := &http.Client{
		Transport: tr,
		Timeout:   time.Second,
	}
	for _, u := range urls {
		resp, err := cc.Get(u + "/members")
		if err != nil {
			if logerr {
				log.Printf("etcdserver: could not get cluster response from %s: %v", u, err)
			}
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			if logerr {
				log.Printf("etcdserver: could not read the body of cluster response: %v", err)
			}
			continue
		}
		var membs []*Member
		if err := json.Unmarshal(b, &membs); err != nil {
			if logerr {
				log.Printf("etcdserver: could not unmarshal cluster response: %v", err)
			}
			continue
		}
		id, err := types.IDFromString(resp.Header.Get("X-Etcd-Cluster-ID"))
		if err != nil {
			if logerr {
				log.Printf("etcdserver: could not parse the cluster ID from cluster res: %v", err)
			}
			continue
		}
		return NewClusterFromMembers("", id, membs), nil
	}
	return nil, fmt.Errorf("etcdserver: could not retrieve cluster information from the given urls")
}

// getRemotePeerURLs returns peer urls of remote members in the cluster. The
// returned list is sorted in ascending lexicographical order.
func getRemotePeerURLs(cl ClusterInfo, local string) []string {
	us := make([]string, 0)
	for _, m := range cl.Members() {
		if m.Name == local {
			continue
		}
		us = append(us, m.PeerURLs...)
	}
	sort.Strings(us)
	return us
}
