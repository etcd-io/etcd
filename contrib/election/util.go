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
	"math/rand"
	"strconv"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/raft/v3/raftpb"
)

func uint64ToString(i uint64) string {
	return strconv.FormatUint(i, 16)
}

type Packet struct {
	msg  raftpb.Message
	time uint64
}

type mockNet struct {
	packetloss int
	latency    int
	maxsize    uint64
	timetick   uint64
	stopc      chan struct{}

	packets   []Packet
	inQueueC  chan raftpb.Message
	outQueueC chan []raftpb.Message

	logger *zap.Logger
}

func newMockNet(packetloss, latency int, stopc <-chan struct{}, logger *zap.Logger) (chan<- raftpb.Message, <-chan []raftpb.Message) {
	var maxsize uint64 = 10000
	inQueueC := make(chan raftpb.Message, maxsize)
	outQueueC := make(chan []raftpb.Message, maxsize)
	mc := &mockNet{
		packetloss: packetloss,
		latency:    latency,
		maxsize:    maxsize,
		timetick:   uint64(0),
		packets:    make([]Packet, 0),
		inQueueC:   inQueueC,
		outQueueC:  outQueueC,
		logger:     logger,
	}
	go mc.serveChannels()
	return inQueueC, outQueueC
}

func (mn *mockNet) sendPackets(pkts []Packet) {
	if len(pkts) > 0 {
		if uint64(len(mn.outQueueC)) > mn.maxsize {
			mn.logger.Warn("messages exceed the capacity of output channel and block")
		}
		msgs := make([]raftpb.Message, 0, len(pkts))
		for i := range pkts {
			msgs = append(msgs, pkts[i].msg)
		}
		mn.outQueueC <- msgs
	}
}

func (mn *mockNet) packetsToSend() (npkts []Packet) {
	if len(mn.packets) > 0 {
		var idx int = 0
		for i := range mn.packets {
			if mn.packets[i].time+uint64(mn.latency) < mn.timetick {
				idx = i
			} else {
				break
			}
		}
		npkts = mn.packets[0 : idx+1]
		mn.packets = mn.packets[idx+1:]
	}
	return npkts
}

func (mn *mockNet) serveChannels() {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			mn.timetick++
			mn.sendPackets(mn.packetsToSend())
		case msg, ok := <-mn.inQueueC:
			if !ok {
				return
			}
			if rand.Int() > mn.packetloss {
				mn.packets = append(mn.packets, Packet{msg: msg, time: mn.timetick})
			}
		case <-mn.stopc:
			close(mn.outQueueC)
		}
	}
}
