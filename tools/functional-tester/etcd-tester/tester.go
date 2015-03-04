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

package main

import (
	"fmt"

	"github.com/coreos/etcd/tools/functional-tester/etcd-agent/client"
)

type tester struct {
	failures []failure
	agents   []client.Agent
	limit    uint64
}

func (tt *tester) runLoop() {
	for i := 0; i < tt.limit; i++ {
		for j, f := range tt.failures {
			fmt.Println("etcd-tester: [round#%d case#%d] start failure %s", i, j, f.Desc())
			fmt.Println("etcd-tester: [round#%d case#%d] start injecting failure...", i, j)
			if err := f.Inject(tt.agents); err != nil {
				fmt.Println("etcd-tester: [round#%d case#%d] injection failing...", i, j)
				tt.cleanup(i, j)
			}
			fmt.Println("etcd-tester: [round#%d case#%d] start recovering failure...", i, j)
			if err := f.Recover(tt.agents); err != nil {
				fmt.Println("etcd-tester: [round#%d case#%d] recovery failing...", i, j)
				tt.cleanup(i, j)
			}
			fmt.Println("etcd-tester: [round#%d case#%d] succeed!", i, j)
		}
	}
}

func (tt *tester) cleanup(i, j int) {
	fmt.Println("etcd-tester: [round#%d case#%d] cleaning up...", i, j)
	for _, a := range tt.agents {
		a.Terminate()
		a.Start()
	}
}
