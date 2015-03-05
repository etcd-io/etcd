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
