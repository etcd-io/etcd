// Copyright 2023 Huidong Zhang, OceanBase, AntGroup
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
	"time"

	"go.uber.org/zap"
)

func main() {
	cluster := flag.String("cluster", "http://127.0.0.1:9021", "comma separated cluster peers")
	id := flag.Int("id", 1, "node ID")
	duration := flag.Duration("duration", 60, "duration of election experiment in seconds")
	packetloss := flag.Int("packetloss", 0, "ratio to trigger packet loss in percentage")
	latency := flag.Int("latency", 0, "additional latency for message transmission")
	flag.Parse()

	stopc := make(chan struct{})
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	inQueueC, outQueueC := newMockNet(*packetloss, *latency, stopc, logger)
	stopdonec := newRaftNode(*id, strings.Split(*cluster, ","), stopc, inQueueC, outQueueC, logger)
	for {
		select {
		case <-time.After(*duration * time.Second):
			close(stopc)
		case <-stopdonec:
			log.Print("election experiment finished")
			return
		}
	}
}
