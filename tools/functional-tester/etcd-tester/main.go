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
	"flag"
	"log"
	"strings"
)

func main() {
	endpointStr := flag.String("agent-endpoints", ":9027", "")
	datadir := flag.String("data-dir", "agent.etcd", "")
	limit := flag.Int("limit", 3, "")
	flag.Parse()

	endpoints := strings.Split(*endpointStr, ",")
	c, err := newCluster(endpoints, *datadir)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Terminate()

	stressers := make([]Stresser, len(c.ClientURLs))
	for i, u := range c.ClientURLs {
		s := &stresser{
			Endpoint: u,
			N:        200,
		}
		go s.Stress()
		stressers[i] = s
	}

	t := &tester{
		failures: []failure{newFailureBase(), newFailureKillAll()},
		cluster:  c,
		limit:    *limit,
	}
	t.runLoop()

	for _, s := range stressers {
		s.Cancel()
	}
}
