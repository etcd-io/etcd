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

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	"github.com/coreos/etcd/pkg/types"
	"github.com/coreos/etcd/version"
)

// isMemberBootstrapped tries to check if the given member has been bootstrapped
// in the given cluster.
func isMemberBootstrapped(cl *cluster, member string, tr *http.Transport) bool {
	rcl, err := getClusterFromRemotePeers(getRemotePeerURLs(cl, member), false, tr)
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

// GetClusterFromRemotePeers takes a set of URLs representing etcd peers, and
// attempts to construct a Cluster by accessing the members endpoint on one of
// these URLs. The first URL to provide a response is used. If no URLs provide
// a response, or a Cluster cannot be successfully created from a received
// response, an error is returned.
func GetClusterFromRemotePeers(urls []string, tr *http.Transport) (*cluster, error) {
	return getClusterFromRemotePeers(urls, true, tr)
}

// If logerr is true, it prints out more error messages.
func getClusterFromRemotePeers(urls []string, logerr bool, tr *http.Transport) (*cluster, error) {
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
		return newClusterFromMembers("", id, membs), nil
	}
	return nil, fmt.Errorf("etcdserver: could not retrieve cluster information from the given urls")
}

// getRemotePeerURLs returns peer urls of remote members in the cluster. The
// returned list is sorted in ascending lexicographical order.
func getRemotePeerURLs(cl Cluster, local string) []string {
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

// getVersions returns the versions of the members in the given cluster.
// The key of the returned map is the member's ID. The value of the returned map
// is the semver version string. If it fails to get the version of a member, the key
// will be an empty string.
func getVersions(cl Cluster, tr *http.Transport) map[string]string {
	members := cl.Members()
	vers := make(map[string]string)
	for _, m := range members {
		ver, err := getVersion(m, tr)
		if err != nil {
			log.Printf("etcdserver: cannot get the version of member %s (%v)", m.ID, err)
			vers[m.ID.String()] = ""
		} else {
			vers[m.ID.String()] = ver
		}
	}
	return vers
}

// decideClusterVersion decides the cluster version based on the versions map.
// The returned version is the min version in the map, or nil if the min
// version in unknown.
func decideClusterVersion(vers map[string]string) *semver.Version {
	var cv *semver.Version
	lv := semver.Must(semver.NewVersion(version.Version))

	for mid, ver := range vers {
		if len(ver) == 0 {
			return nil
		}
		v, err := semver.NewVersion(ver)
		if err != nil {
			log.Printf("etcdserver: cannot understand the version of member %s (%v)", mid, err)
			return nil
		}
		if lv.LessThan(*v) {
			log.Printf("etcdserver: the etcd version %s is not up-to-date", lv.String())
			log.Printf("etcdserver: member %s has a higher version %s", mid, ver)
		}
		if cv == nil {
			cv = v
		} else if v.LessThan(*cv) {
			cv = v
		}
	}
	return cv
}

// getVersion returns the version of the given member via its
// peerURLs. Returns the last error if it fails to get the version.
func getVersion(m *Member, tr *http.Transport) (string, error) {
	cc := &http.Client{
		Transport: tr,
		Timeout:   time.Second,
	}
	var (
		err  error
		resp *http.Response
	)

	for _, u := range m.PeerURLs {
		resp, err = cc.Get(u + "/version")
		if err != nil {
			continue
		}
		b, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		var vers version.Versions
		if err := json.Unmarshal(b, &vers); err != nil {
			continue
		}
		return vers.Server, nil
	}
	return "", err
}
