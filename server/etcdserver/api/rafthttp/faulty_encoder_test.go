// Copyright 2015 The etcd Authors
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

package rafthttp

import (
	"reflect"
	"testing"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/raft/v3/raftpb"
)

type testEncoder struct {
	received     []*raftpb.Message
	receivedTime []time.Time
}

func newTestEncoder() *testEncoder {
	return &testEncoder{
		received:     make([]*raftpb.Message, 0),
		receivedTime: make([]time.Time, 0),
	}
}

func (te *testEncoder) encode(m *raftpb.Message) error {
	te.received = append(te.received, m)
	te.receivedTime = append(te.receivedTime, time.Now())
	return nil
}

type testEncodeResult struct {
	received     []*raftpb.Message
	receivedTime []time.Time
	sent         []*raftpb.Message
	sentTime     []time.Time
}

func (ter *testEncodeResult) hasReordered() bool {
	receivedOrders := map[*raftpb.Message]int{}
	for i, m := range ter.received {
		receivedOrders[m] = i
	}

	lastReceiveIndex := -1
	for _, m := range ter.sent {
		if i, ok := receivedOrders[m]; ok {
			if i < lastReceiveIndex {
				return true
			}
			lastReceiveIndex = i
		}
	}

	return false
}

func (ter *testEncodeResult) hasDropped() bool {
	receivedOrders := map[*raftpb.Message]int{}
	for i, m := range ter.received {
		receivedOrders[m] = i
	}

	for _, m := range ter.sent {
		if _, ok := receivedOrders[m]; !ok {
			return true
		}
	}

	return false
}

func (ter *testEncodeResult) hasDuplicated() bool {
	receivedOrders := map[*raftpb.Message]int{}
	for i, m := range ter.received {
		if _, ok := receivedOrders[m]; !ok {
			receivedOrders[m] = i
		} else {
			return true
		}
	}

	return false
}

func (ter *testEncodeResult) hasConsistentCountAndOrder() bool {
	if len(ter.sent) != len(ter.received) {
		return false
	}

	for i, m := range ter.sent {
		if ter.received[i] != m {
			return false
		}
	}

	return true
}

func (ter *testEncodeResult) hasBlocked(threshold time.Duration) bool {
	if ter.receivedTime[0].Sub(ter.sentTime[0]) < threshold {
		return true
	}

	prevReceivedTime := ter.receivedTime[0]
	for i := range ter.received {
		if ter.receivedTime[i].Sub(ter.sentTime[i]) > threshold && ter.receivedTime[i].Sub(prevReceivedTime) < threshold {
			return true
		}
	}

	return false
}

func (ter *testEncodeResult) hasDelayed(threshold time.Duration) bool {
	sentTime := map[*raftpb.Message]time.Time{}
	for i, m := range ter.sent {
		sentTime[m] = ter.sentTime[i]
	}

	for i, m := range ter.received {
		if ter.receivedTime[i].Sub(sentTime[m]) > threshold {
			return true
		}
	}

	return false
}

func runWithConfig(t *testing.T, cfg FaultyNetworkConfig, nSend int, sendInterval time.Duration, target uint64) *testEncodeResult {
	sendMessages := []*raftpb.Message{}
	for i := 0; i < nSend; i++ {
		sendMessages = append(sendMessages, &raftpb.Message{To: target})
	}

	enc := newTestEncoder()
	fe := newFaultyEncoder(enc, types.ID(0), types.ID(target), zap.NewNop())
	fe.setConfig(cfg.String())

	result := &testEncodeResult{}
	for _, m := range sendMessages {
		result.sent = append(result.sent, m)
		result.sentTime = append(result.sentTime, time.Now())
		fe.encode(m)
		time.Sleep(sendInterval)
	}

	// wait till send queue is empty (at most 30 seconds)
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			t.Fatal("timeout in running faulty network tests")
			return nil
		default:
			time.Sleep(time.Millisecond * 100)
		}
		pending := fe.getPendingStatus()
		if pending.nMessages == 0 {
			break
		}
	}

	result.received = enc.received
	result.receivedTime = enc.receivedTime

	fe.Close()

	return result
}

func TestFaultyEncoderWithBlockedNetwork(t *testing.T) {
	tr := FaultyNetworkTransport{
		From: 0,
		To:   1,
	}
	cfg := FaultyNetworkConfig{
		tr: FaultyNetworkFaultConfig{
			BlockInSecond: 0.2,
		},
	}

	result := runWithConfig(t, cfg, 3, 0, 1)
	if !result.hasConsistentCountAndOrder() {
		t.Error("inconsistent count or order")
	}
	if !result.hasBlocked(time.Millisecond * 100) {
		t.Error("messages shall be blocked")
	}
}

func TestFaultyEncoderWithLossyNetwork(t *testing.T) {
	tr := FaultyNetworkTransport{
		From: 0,
		To:   2,
	}
	cfg := FaultyNetworkConfig{
		tr: FaultyNetworkFaultConfig{
			DropPropability: 0.2,
		},
	}

	result := runWithConfig(t, cfg, 20, 0, 2) // 99% confidence level
	if !result.hasDropped() {
		t.Error("some messages shall be dropped")
	}
}

func TestFaultyEncoderWithLaggyNetwork(t *testing.T) {
	tr := FaultyNetworkTransport{
		From: 0,
		To:   1,
	}
	cfg := FaultyNetworkConfig{
		tr: FaultyNetworkFaultConfig{
			DelayProbability: 0.5,
			MinDelayInSecond: 0.5,
			MaxDelayInSecond: 1,
		},
	}

	result := runWithConfig(t, cfg, 20, time.Millisecond*100, 1) // 99% confidence level
	if !result.hasDelayed(time.Millisecond * 40) {
		t.Error("some messages shall be delayed")
	}
}

func TestFaultyEncoderWithDuplicatedMessages(t *testing.T) {
	tr := FaultyNetworkTransport{
		From: 0,
		To:   1,
	}
	cfg := FaultyNetworkConfig{
		tr: FaultyNetworkFaultConfig{
			DuplicateProbability: 0.2,
		},
	}

	result := runWithConfig(t, cfg, 20, 0, 1) // 99% confidence level
	if !result.hasDuplicated() {
		t.Error("some messages shall be duplicated")
	}
}

func TestFaultyNetworkWithReorderedMessages(t *testing.T) {
	tr := FaultyNetworkTransport{
		From: 0,
		To:   1,
	}
	cfg := FaultyNetworkConfig{
		tr: FaultyNetworkFaultConfig{
			DelayProbability: 0.2,
			MinDelayInSecond: 1,
			MaxDelayInSecond: 1,
		},
	}

	result := runWithConfig(t, cfg, 20, 0, 1) // 99% confidence level
	if !result.hasReordered() {
		t.Error("some messages shall be delayed")
	}
}

func TestFaultyNetworkWithMismatchedConfig(t *testing.T) {
	// faulty network is applied for messages sent to node 2
	tr := FaultyNetworkTransport{
		From: 0,
		To:   2,
	}
	cfg := FaultyNetworkConfig{
		tr: FaultyNetworkFaultConfig{
			DropPropability: 1,
		},
	}

	result := runWithConfig(t, cfg, 10, 0, 1)
	if !result.hasConsistentCountAndOrder() {
		t.Error("received messages are not aligned with sent ones")
	}
}

func TestFaultyNetworkConfigure(t *testing.T) {
	tr1 := FaultyNetworkTransport{
		From: 1,
		To:   2,
	}
	tr2 := FaultyNetworkTransport{
		From:   3,
		To:     2,
		Duplex: true,
	}
	tr3 := FaultyNetworkTransport{
		From: 1,
		To:   0,
	}
	tr4 := FaultyNetworkTransport{
		From: 0,
		To:   0,
	}
	cfg := FaultyNetworkConfig{
		tr1: FaultyNetworkFaultConfig{
			DropPropability:      0.1,
			DuplicateProbability: 0.2,
		},
		tr2: FaultyNetworkFaultConfig{
			BlockInSecond:    1,
			DelayProbability: 0.5,
			MinDelayInSecond: 2,
			MaxDelayInSecond: 3,
		},
		tr3: FaultyNetworkFaultConfig{
			DropPropability: 0.5,
		},
		tr4: FaultyNetworkFaultConfig{
			DropPropability: 0.1,
		},
	}

	cfgStr := cfg.String()
	newCfg := FaultyNetworkConfig{}
	if err := newCfg.Parse(cfgStr); err != nil {
		t.Errorf("failed to parse config. error: %s", err)
	}

	if !reflect.DeepEqual(cfg, newCfg) {
		t.Error("configure mismatch")
	}
}
