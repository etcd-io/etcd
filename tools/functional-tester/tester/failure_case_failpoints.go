// Copyright 2018 The etcd Authors
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

package tester

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/tools/functional-tester/rpcpb"
)

type failpointStats struct {
	mu sync.Mutex
	// crashes counts the number of crashes for a failpoint
	crashes map[string]int
}

var fpStats failpointStats

func failpointFailures(clus *Cluster) (ret []Failure, err error) {
	var fps []string
	fps, err = failpointPaths(clus.Members[0].FailpointHTTPAddr)
	if err != nil {
		return nil, err
	}
	// create failure objects for all failpoints
	for _, fp := range fps {
		if len(fp) == 0 {
			continue
		}

		fpFails := failuresFromFailpoint(fp, clus.Tester.FailpointCommands)

		// wrap in delays so failpoint has time to trigger
		for i, fpf := range fpFails {
			if strings.Contains(fp, "Snap") {
				// hack to trigger snapshot failpoints
				fpFails[i] = &failureUntilSnapshot{
					desc:        desc(fpf.Desc()),
					failureCase: rpcpb.FailureCase_FAILPOINTS,
					Failure:     fpf,
				}
			} else {
				fpFails[i] = &failureDelay{
					Failure:       fpf,
					delayDuration: 3 * time.Second,
				}
			}
		}
		ret = append(ret, fpFails...)
	}
	fpStats.crashes = make(map[string]int)
	return ret, err
}

func failpointPaths(endpoint string) ([]string, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, rerr := ioutil.ReadAll(resp.Body)
	if rerr != nil {
		return nil, rerr
	}
	var fps []string
	for _, l := range strings.Split(string(body), "\n") {
		fp := strings.Split(l, "=")[0]
		fps = append(fps, fp)
	}
	return fps, nil
}

// failpoints follows FreeBSD KFAIL_POINT syntax.
// e.g. panic("etcd-tester"),1*sleep(1000)->panic("etcd-tester")
func failuresFromFailpoint(fp string, failpointCommands []string) (fs []Failure) {
	recov := makeRecoverFailpoint(fp)
	for _, fcmd := range failpointCommands {
		inject := makeInjectFailpoint(fp, fcmd)
		fs = append(fs, []Failure{
			&failureFollower{
				failureByFunc: failureByFunc{
					desc:          desc(fmt.Sprintf("failpoint %q (one: %q)", fp, fcmd)),
					failureCase:   rpcpb.FailureCase_FAILPOINTS,
					injectMember:  inject,
					recoverMember: recov,
				},
				last: -1,
				lead: -1,
			},
			&failureLeader{
				failureByFunc: failureByFunc{
					desc:          desc(fmt.Sprintf("failpoint %q (leader: %q)", fp, fcmd)),
					failureCase:   rpcpb.FailureCase_FAILPOINTS,
					injectMember:  inject,
					recoverMember: recov,
				},
				last: -1,
				lead: -1,
			},
			&failureQuorum{
				desc:          desc(fmt.Sprintf("failpoint %q (quorum: %q)", fp, fcmd)),
				failureCase:   rpcpb.FailureCase_FAILPOINTS,
				injectMember:  inject,
				recoverMember: recov,
			},
			&failureAll{
				desc:          desc(fmt.Sprintf("failpoint %q (all: %q)", fp, fcmd)),
				failureCase:   rpcpb.FailureCase_FAILPOINTS,
				injectMember:  inject,
				recoverMember: recov,
			},
		}...)
	}
	return fs
}

func makeInjectFailpoint(fp, val string) injectMemberFunc {
	return func(clus *Cluster, idx int) (err error) {
		return putFailpoint(clus.Members[idx].FailpointHTTPAddr, fp, val)
	}
}

func makeRecoverFailpoint(fp string) recoverMemberFunc {
	return func(clus *Cluster, idx int) error {
		if err := delFailpoint(clus.Members[idx].FailpointHTTPAddr, fp); err == nil {
			return nil
		}
		// node not responding, likely dead from fp panic; restart
		fpStats.mu.Lock()
		fpStats.crashes[fp]++
		fpStats.mu.Unlock()
		return recoverKill(clus, idx)
	}
}

func putFailpoint(ep, fp, val string) error {
	req, _ := http.NewRequest(http.MethodPut, ep+"/"+fp, strings.NewReader(val))
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("failed to PUT %s=%s at %s (%v)", fp, val, ep, resp.Status)
	}
	return nil
}

func delFailpoint(ep, fp string) error {
	req, _ := http.NewRequest(http.MethodDelete, ep+"/"+fp, strings.NewReader(""))
	c := http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("failed to DELETE %s at %s (%v)", fp, ep, resp.Status)
	}
	return nil
}
