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

import "math/rand"

type failureKillMajority struct {
	description
}

func newFailureKillMajority() *failureKillMajority {
	return &failureKillMajority{
		description: "kill majority of the cluster",
	}
}

func (f *failureKillMajority) Inject(c *cluster, round int) error {
	for i := range getToKillMap(c.Size, round) {
		if err := c.Agents[i].Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureKillMajority) Recover(c *cluster, round int) error {
	for i := range getToKillMap(c.Size, round) {
		if _, err := c.Agents[i].Restart(); err != nil {
			return err
		}
	}
	return c.WaitHealth()
}

func getToKillMap(size int, seed int) map[int]bool {
	m := make(map[int]bool)
	r := rand.New(rand.NewSource(int64(seed)))
	majority := size/2 + 1
	for {
		m[r.Intn(size)] = true
		if len(m) >= majority {
			return m
		}
	}
}
