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
	"math/rand"
	"time"
)

const snapshotCount = 10000

type failure interface {
	// Inject injeccts the failure into the testing cluster at the given
	// round. When calling the function, the cluster should be in health.
	Inject(c *cluster, round int) error
	// Recover recovers the injected failure caused by the injection of the
	// given round and wait for the recovery of the testing cluster.
	Recover(c *cluster, round int) error
	// return a description of the failure
	Desc() string
}

type description string

func (d description) Desc() string { return string(d) }

type failureKillAll struct {
	description
}

func newFailureKillAll() *failureKillAll {
	return &failureKillAll{
		description: "kill all members",
	}
}

func (f *failureKillAll) Inject(c *cluster, round int) error {
	for _, a := range c.Agents {
		if err := a.Stop(); err != nil {
			return err
		}
	}
	return nil
}

func (f *failureKillAll) Recover(c *cluster, round int) error {
	for _, a := range c.Agents {
		if _, err := a.Restart(); err != nil {
			return err
		}
	}
	return c.WaitHealth()
}

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

type failureKillOne struct {
	description
}

func newFailureKillOne() *failureKillOne {
	return &failureKillOne{
		description: "kill one random member",
	}
}

func (f *failureKillOne) Inject(c *cluster, round int) error {
	i := round % c.Size
	return c.Agents[i].Stop()
}

func (f *failureKillOne) Recover(c *cluster, round int) error {
	i := round % c.Size
	if _, err := c.Agents[i].Restart(); err != nil {
		return err
	}
	return c.WaitHealth()
}

// failureKillOneForLongTime kills one member for long time, and restart
// after a snapshot is required.
type failureKillOneForLongTime struct {
	description
}

func newFailureKillOneForLongTime() *failureKillOneForLongTime {
	return &failureKillOneForLongTime{
		description: "kill one member for long time and expect it to recover from incoming snapshot",
	}
}

func (f *failureKillOneForLongTime) Inject(c *cluster, round int) error {
	i := round % c.Size
	if err := c.Agents[i].Stop(); err != nil {
		return err
	}
	if c.Size >= 3 {
		start, _ := c.Report()
		var end int
		// Normal healthy cluster could accept 1000req/s at least.
		// Give it 3-times time to create a new snapshot.
		retry := snapshotCount / 1000 * 3
		for j := 0; j < retry; j++ {
			end, _ = c.Report()
			// If the number of proposals committed is bigger than snapshot count,
			// a new snapshot should have been created.
			if end-start > snapshotCount {
				return nil
			}
			time.Sleep(time.Second)
		}
		return fmt.Errorf("cluster too slow: only commit %d requests in %ds", end-start, retry)
	}
	return nil
}

func (f *failureKillOneForLongTime) Recover(c *cluster, round int) error {
	i := round % c.Size
	if _, err := c.Agents[i].Restart(); err != nil {
		return err
	}
	return c.WaitHealth()
}
