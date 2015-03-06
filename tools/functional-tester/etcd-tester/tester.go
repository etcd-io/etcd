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

import "log"

type tester struct {
	failures []failure
	cluster  *cluster
	limit    int
}

func (tt *tester) runLoop() {
	for i := 0; i < tt.limit; i++ {
		for j, f := range tt.failures {
			if err := tt.cluster.WaitHealth(); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] wait full health error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] start failure %s", i, j, f.Desc())
			log.Printf("etcd-tester: [round#%d case#%d] start injecting failure...", i, j)
			if err := f.Inject(tt.cluster, i); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] injection error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] start recovering failure...", i, j)
			if err := f.Recover(tt.cluster, i); err != nil {
				log.Printf("etcd-tester: [round#%d case#%d] recovery error: %v", i, j, err)
				if err := tt.cleanup(i, j); err != nil {
					log.Printf("etcd-tester: [round#%d case#%d] cleanup error: %v", i, j, err)
					return
				}
				continue
			}
			log.Printf("etcd-tester: [round#%d case#%d] succeed!", i, j)
		}
	}
}

func (tt *tester) cleanup(i, j int) error {
	log.Printf("etcd-tester: [round#%d case#%d] cleaning up...", i, j)
	if err := tt.cluster.Cleanup(); err != nil {
		return err
	}
	return tt.cluster.Bootstrap()
}
