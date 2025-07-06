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
	"strings"

	"go.etcd.io/etcd/tests/v3/functional/rpcpb"
)

const (
	diskIOFailpoint = "raftAfterSave"
)

func failpointDiskIOFailures(clus *Cluster) (ret []Case, err error) {
	fps, err := failpointPaths(clus.Members[0].FailpointHTTPAddr)
	if err != nil {
		return nil, err
	}
	var detailDiskIOLatencyFailpointPath string
	for i := 0; i < len(fps); i++ {
		if strings.HasSuffix(fps[i], diskIOFailpoint) {
			detailDiskIOLatencyFailpointPath = fps[i]
			break
		}
	}
	// create failure objects for diskIOFailpoint
	fpFails := casesFromDiskIOFailpoint(detailDiskIOLatencyFailpointPath, clus.Tester.FailpointCommands)
	// wrap in delays so failpoint has time to trigger
	for i, fpf := range fpFails {
		fpFails[i] = &caseDelay{
			Case:          fpf,
			delayDuration: clus.GetCaseDelayDuration(),
		}
	}
	ret = append(ret, fpFails...)
	return ret, nil
}

func casesFromDiskIOFailpoint(fp string, failpointCommands []string) (fs []Case) {
	recov := makeRecoverFailpoint(fp)
	for _, fcmd := range failpointCommands {
		inject := makeInjectFailpoint(fp, fcmd)
		fs = append(fs, []Case{
			&caseLeader{
				caseByFunc: caseByFunc{
					desc:          fmt.Sprintf("failpoint %q (leader: %q)", fp, fcmd),
					rpcpbCase:     rpcpb.Case_FAILPOINTS,
					injectMember:  inject,
					recoverMember: recov,
				},
				last: -1,
				lead: -1,
			},
		}...)
	}
	return fs
}
