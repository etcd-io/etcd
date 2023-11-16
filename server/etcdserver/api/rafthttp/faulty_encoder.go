// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rafthttp

import (
	"container/heap"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/raft/v3/raftpb"
)

type FaultType int

const (
	FaultTypeNone       FaultType = 0
	FaultTypeDropped    FaultType = 0x02
	FaultTypeDuplicated FaultType = 0x04
	FaultTypeDelayed    FaultType = 0x08
	FaultTypeReordered  FaultType = 0x10
	FaultTypeBlocked    FaultType = 0x12
)

type FaultyNetworkFaultConfig struct {
	DropPropability      float64
	DuplicateProbability float64
	DelayProbability     float64
	MinDelayInSecond     float64
	MaxDelayInSecond     float64
	BlockInSecond        float64
}

// FaultyNetworkTransport defines a peer-peer transport between two nodes specified by node ids.
// zero node id refers to any node.
// when duplex is true, the transport can be from->to or to->from.
type FaultyNetworkTransport struct {
	From   uint64
	To     uint64
	Duplex bool
}

type FaultyNetworkConfig map[FaultyNetworkTransport]FaultyNetworkFaultConfig
type FaultyNetworkEffectiveConfig map[uint64]FaultyNetworkFaultConfig

type FaultStat struct {
	Dropped    uint64 `json:"dropped,omitempty"`
	Duplicated uint64 `json:"duplicated,omitempty"`
	Delayed    uint64 `json:"delayed,omitempty"`
	Reordered  uint64 `json:"reordered,omitempty"`
}

var reportFaultStatisticsInterval = time.Second * 10

// String converts config to string format
// example: 111->222:drop=0.1,delay=0.2:1-3;456:block=2;333<->*:dup:0.2;*:dup=0.05
// explain:
// 1. messages sent from node 111 to node 222 will be dropped with 10% chance, be delayed by 1 to 3 seconds with 20% chance, and be bocked by 2 seconds.
// 2. messages sent from or to 333, will be duplicated with 20% chance.
// 3. for all transports that are specified above, a message may be duplicated with 5% chance
func (fnc *FaultyNetworkConfig) String() string {
	cfgStrs := []string{}
	for tr, p := range *fnc {
		pstrs := []string{}
		if p.DropPropability != 0 {
			pstrs = append(pstrs, fmt.Sprintf("drop=%v", p.DropPropability))
		}
		if p.DuplicateProbability != 0 {
			pstrs = append(pstrs, fmt.Sprintf("dup=%v", p.DuplicateProbability))
		}
		if p.DelayProbability != 0 {
			pstrs = append(pstrs, fmt.Sprintf("delay=%v:%v-%v", p.DelayProbability, p.MinDelayInSecond, p.MaxDelayInSecond))
		}
		if p.BlockInSecond != 0 {
			pstrs = append(pstrs, fmt.Sprintf("block=%v", p.BlockInSecond))
		}

		cfgStrs = append(cfgStrs, fmt.Sprintf("%s:%s", fnc.formatTransport(tr), strings.Join(pstrs, ",")))
	}

	return strings.Join(cfgStrs, ";")
}

// Parse parses FaultyNetworkConfig from string.
// refer to FaultyNetworkConfig.String() for configure format.
func (fnc *FaultyNetworkConfig) Parse(str string) error {
	cfg := FaultyNetworkConfig{}
	cfgTokens := strings.Split(str, ";")
	for _, ct := range cfgTokens {
		trStr, pstr, found := strings.Cut(ct, ":")
		if !found {
			return fmt.Errorf("invalid faulty network config")
		}

		p := FaultyNetworkFaultConfig{}
		var err error
		propTokens := strings.Split(pstr, ",")
		for _, pt := range propTokens {
			key, val, found := strings.Cut(pt, "=")
			if !found {
				return fmt.Errorf("invalid fault config key value: %s", pt)
			}
			switch key {
			case "drop":
				if p.DropPropability, err = strconv.ParseFloat(val, 64); err != nil {
					return fmt.Errorf("invalid drop probability %v for node %v: %w", key, val, err)
				}
			case "dup":
				if p.DuplicateProbability, err = strconv.ParseFloat(val, 64); err != nil {
					return fmt.Errorf("invalid dup probability %v for node %v: %w", key, val, err)
				}
			case "block":
				if p.BlockInSecond, err = strconv.ParseFloat(val, 64); err != nil {
					return fmt.Errorf("invalid block duration %v for node %v: %w", key, val, err)
				}
			case "delay":
				pDelayStr, delayRangeStr, found := strings.Cut(val, ":")
				if !found {
					return fmt.Errorf("invalid delay configure: %s", val)
				}
				if p.DelayProbability, err = strconv.ParseFloat(pDelayStr, 64); err != nil {
					return fmt.Errorf("invalid delay probability %v for node %v: %w", key, val, err)
				}
				minDelay, maxDelay, found := strings.Cut(delayRangeStr, "-")
				if !found {
					return fmt.Errorf("invalid delay range: %s", delayRangeStr)
				}
				if p.MinDelayInSecond, err = strconv.ParseFloat(minDelay, 64); err != nil {
					return fmt.Errorf("invalid delay probability %v for node %v: %w", key, val, err)
				}
				if p.MaxDelayInSecond, err = strconv.ParseFloat(maxDelay, 64); err != nil {
					return fmt.Errorf("invalid delay probability %v for node %v: %w", key, val, err)
				}
			}
		}

		tr, err := fnc.parseTransport(trStr)
		if err != nil {
			return fmt.Errorf("invalid transport string %s. error: %w", trStr, err)
		}

		cfg[tr] = p
	}

	*fnc = cfg
	return nil
}

func (fnc *FaultyNetworkConfig) parseNodeId(nstr string) (uint64, error) {
	if nstr == "*" {
		return 0, nil
	}

	if i, err := strconv.ParseUint(nstr, 10, 64); err == nil {
		// try to parse not from dec format
		return i, nil
	}

	if i, err := strconv.ParseUint(nstr, 16, 64); err == nil {
		// try to parse not from hex format
		return i, nil
	}

	return 0, fmt.Errorf("invalid node id %s", nstr)
}

func (fnc *FaultyNetworkConfig) parseTransport(str string) (FaultyNetworkTransport, error) {
	if str == "*" {
		return FaultyNetworkTransport{
			From: 0,
			To:   0,
		}, nil
	}

	directions := []string{"<->", "->"}
	for _, direction := range directions {
		if from, to, found := strings.Cut(str, direction); found {
			var err error
			tr := FaultyNetworkTransport{}
			if tr.From, err = fnc.parseNodeId(from); err != nil {
				return tr, err
			}
			if tr.To, err = fnc.parseNodeId(to); err != nil {
				return tr, err
			}
			if direction == "<->" {
				tr.Duplex = true
			} else {
				tr.Duplex = false
			}

			return tr, nil
		}
	}

	return FaultyNetworkTransport{}, fmt.Errorf("failed to parse transport %s", str)
}

func (fnc *FaultyNetworkConfig) formatTransport(tr FaultyNetworkTransport) string {
	if tr.From == 0 && tr.To == 0 {
		return "*"
	}

	var from, to, direction string
	if tr.From == 0 {
		from = "*"
	} else {
		from = strconv.FormatUint(tr.From, 16)
	}
	if tr.To == 0 {
		to = "*"
	} else {
		to = strconv.FormatUint(tr.To, 16)
	}
	if tr.Duplex {
		direction = "<->"
	} else {
		direction = "->"
	}

	return fmt.Sprintf("%s%s%s", from, direction, to)
}

type msgItem struct {
	sendAt time.Time
	m      *raftpb.Message
	index  uint64
}

// priority queue that sort items by their sendAt time.
type msgPriorityQueue []*msgItem

func (pq msgPriorityQueue) Len() int { return len(pq) }

func (pq msgPriorityQueue) Less(i, j int) bool {
	return pq[i].sendAt.Before(pq[j].sendAt)
}

func (pq msgPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *msgPriorityQueue) Push(x any) {
	item := x.(*msgItem)
	*pq = append(*pq, item)
}

func (pq *msgPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	*pq = old[0 : n-1]
	return item
}

// sendQueue buffers delayed messages in a priority queue.
// each message is associated with a sendAt time when the
// message shall be sent.
type sendQueue struct {
	q msgPriorityQueue
}

func newSendQueue() *sendQueue {
	return &sendQueue{
		q: make(msgPriorityQueue, 0),
	}
}

func (sq *sendQueue) len() int {
	return len(sq.q)
}

func (sq *sendQueue) push(di *msgItem) {
	heap.Push(&sq.q, di)
}

func (sq *sendQueue) pop() *msgItem {
	return heap.Pop(&sq.q).(*msgItem)
}

func (sq *sendQueue) peek() *msgItem {
	n := sq.q.Len()
	if n <= 0 {
		return nil
	}

	return sq.q[n-1]
}

func (sq *sendQueue) popUntil(t time.Time) []*msgItem {
	result := []*msgItem{}

	for {
		if top := sq.peek(); top != nil && top.sendAt.Before(t) {
			result = append(result, sq.pop())
		} else {
			break
		}
	}

	return result
}

type pendingStatusRequest struct {
	c chan<- pendingStatus
}
type pendingStatus struct {
	nMessages int
}

type faultMessage struct {
	m        *raftpb.Message
	fault    FaultType
	duration time.Duration //<- only used for delayed or blocked message
}

// faultEncoder is a wrapper of real encoder. It applies specified faults
// to the received messages and sent via the wrapped encoder.
type faultyEncoder struct {
	localId uint64
	peerId  uint64

	// original encoder
	encoder encoder

	// priority queue for delayed messages
	sendQueue *sendQueue

	// message index
	currentIndex uint64
	maxSentIndex uint64

	msgc   chan *faultMessage
	errorc chan error
	pendc  chan pendingStatusRequest
	stopc  chan struct{}

	stat   map[string]*FaultStat
	logger *zap.Logger

	config    *FaultyNetworkFaultConfig
	configStr string
}

//nolint:unused // wrapperCloser is used only when gofail is enabled
type wrappedCloser struct {
	fe     *faultyEncoder
	closer io.Closer
}

// Close closes the faulty encoder followed by attached closer
//
//nolint:unused // Close is used only when gofail is enabled
func (wc *wrappedCloser) Close() error {
	wc.fe.Close()
	return wc.closer.Close()
}

// wrapEncoderWithFaultyNetwork wraps the given encoder and closer so that specified faults can be applied to the messages
//
//nolint:unused // wrapEncoderWithFaultyNetwork is used only when gofail is enabled
func wrapEncoderWithFaultyNetwork(encoder encoder, closer io.Closer, localId types.ID, perrId types.ID, lg *zap.Logger) (encoder, io.Closer) {
	fe := newFaultyEncoder(encoder, localId, perrId, lg)
	return fe, &wrappedCloser{fe: fe, closer: closer}
}

func newFaultyEncoder(encoder encoder, localId types.ID, perrId types.ID, lg *zap.Logger) *faultyEncoder {
	fe := &faultyEncoder{
		localId:   uint64(localId),
		peerId:    uint64(perrId),
		encoder:   encoder,
		sendQueue: newSendQueue(),
		msgc:      make(chan *faultMessage),
		errorc:    make(chan error),
		pendc:     make(chan pendingStatusRequest),
		stopc:     make(chan struct{}),
		stat:      make(map[string]*FaultStat),
		logger:    lg,
		config:    nil,
	}

	go fe.sendLoop()

	return fe
}

func (fe *faultyEncoder) encode(m *raftpb.Message) error {
	fe.updateConfigFromGofail()

	if fe.config == nil {
		return fe.encoder.encode(m)
	}

	// block message
	if fe.config.BlockInSecond > 0 {
		return fe.sendWithFault(m, FaultTypeBlocked)
	}

	// drop message
	if rand.Float64() < fe.config.DropPropability {
		return fe.sendWithFault(m, FaultTypeDropped)
	}

	// delayed message
	if rand.Float64() < fe.config.DelayProbability {
		return fe.sendWithFault(m, FaultTypeDelayed)
	}

	// duplicated message. Proposal shall be ignored as it is equivalent to an untracking put request.
	if rand.Float64() < fe.config.DuplicateProbability && m.Type != raftpb.MsgProp {
		if rand.Float64() < fe.config.DelayProbability {
			fe.sendWithFault(m, FaultTypeDelayed|FaultTypeDuplicated)
		} else {
			fe.sendWithFault(m, FaultTypeNone|FaultTypeDuplicated)
		}
	}

	// send message without fault
	return fe.sendWithFault(m, FaultTypeNone)
}

func (fe *faultyEncoder) Close() error {
	close(fe.stopc)
	return nil
}

// faultyEncoder gets realtime fault configuration from gofail failpoints.
func (fe *faultyEncoder) updateConfigFromGofail() {
	// gofail: var faultyNetworkCfg string
	// fe.setConfig(faultyNetworkCfg)
	// return
}

func (fe *faultyEncoder) sendWithFault(m *raftpb.Message, faultType FaultType) error {
	var delay time.Duration
	switch faultType {
	case FaultTypeDelayed:
		delay = time.Duration(float64(time.Second) * (rand.Float64()*(fe.config.MaxDelayInSecond-fe.config.MinDelayInSecond) + fe.config.MinDelayInSecond))
	case FaultTypeBlocked:
		delay = time.Duration(float64(time.Second) * fe.config.BlockInSecond)
	}

	fe.msgc <- &faultMessage{
		m:        m,
		fault:    faultType,
		duration: delay,
	}

	err := <-fe.errorc
	return err
}

func (fe *faultyEncoder) getPendingStatus() pendingStatus {
	c := make(chan pendingStatus)
	fe.pendc <- pendingStatusRequest{c: c}
	r := <-c

	return r
}

// get effective fault configuration that can be applied to this encoder.
// using maximum probability/duration that is applicable
// TODO: calculate effective config so that fault configuration for more specifically matching transport can be applied.
func (fe *faultyEncoder) getEffectiveConfig(cfg FaultyNetworkConfig) *FaultyNetworkFaultConfig {
	fc := &FaultyNetworkFaultConfig{}
	for tr, p := range cfg {
		if ((fe.localId == tr.From || tr.From == 0) || ((fe.localId == tr.To || tr.To == 0) && tr.Duplex)) &&
			((fe.peerId == tr.To || tr.To == 0) || ((fe.peerId == tr.From || tr.From == 0) && tr.Duplex)) {
			fc.BlockInSecond = math.Max(fc.BlockInSecond, p.BlockInSecond)
			fc.DuplicateProbability = math.Max(fc.DuplicateProbability, p.DuplicateProbability)
			fc.DropPropability = math.Max(fc.DropPropability, p.DropPropability)
			if p.DelayProbability > fc.DelayProbability {
				fc.DelayProbability = p.DelayProbability
				fc.MinDelayInSecond = p.MinDelayInSecond
				fc.MaxDelayInSecond = p.MaxDelayInSecond
			}
		}
	}

	return fc
}

func (fe *faultyEncoder) setConfig(cfgStr string) {
	cfg := FaultyNetworkConfig{}
	if cfgStr != fe.configStr {
		if err := cfg.Parse(cfgStr); err != nil {
			fe.logger.Error("Failed to parse faulty network config", zap.String("config", cfgStr), zap.Error(err))
			cfg = nil
		}
		if cfg != nil {
			fe.configStr = cfgStr
			fe.config = fe.getEffectiveConfig(cfg)
		}

	}
}

func (fe *faultyEncoder) recordFault(msgType raftpb.MessageType, faultType FaultType) {
	if faultType == FaultTypeNone {
		return
	}

	typeStr := msgType.String()
	if _, ok := fe.stat[typeStr]; !ok {
		fe.stat[typeStr] = &FaultStat{}
	}
	switch faultType {
	case FaultTypeDropped:
		fe.stat[typeStr].Dropped++
	case FaultTypeDuplicated:
		fe.stat[typeStr].Duplicated++
	case FaultTypeDelayed:
		fe.stat[typeStr].Delayed++
	case FaultTypeReordered:
		fe.stat[typeStr].Reordered++
	}
}

func (fe *faultyEncoder) reportFaultStat() {
	if len(fe.stat) > 0 {
		fe.logger.Info("Faulty encoder statistics", zap.Uint64("id", fe.peerId), zap.Any("stat", fe.stat))
	}
}

func (fe *faultyEncoder) send(m *raftpb.Message, index uint64) error {
	if m == nil {
		return nil
	}

	if index > fe.maxSentIndex {
		fe.maxSentIndex = index
	}
	return fe.encoder.encode(m)
}

func (fe *faultyEncoder) sendLoop() {
	timer := time.NewTimer(time.Minute)
	statTicker := time.NewTicker(reportFaultStatisticsInterval)
	var notifyc <-chan time.Time
	var lastIndex uint64
	for {

		first := fe.sendQueue.peek()
		if first != nil {
			if lastIndex != first.index {
				lastIndex = first.index
				// reset timer
				timer.Stop()
				select {
				case <-timer.C:
				default:
				}
				duration := time.Until(first.sendAt)
				timer.Reset(duration)
				notifyc = timer.C
			}
		} else {
			notifyc = nil
		}

		select {
		case fm := <-fe.msgc:
			isDup := false
			fault := fm.fault
			if fault&FaultTypeDuplicated == FaultTypeDuplicated {
				isDup = true
				fault = fault & ^FaultTypeDuplicated
			}
			var err error
			switch fault {
			case FaultTypeNone:
				err = fe.send(fm.m, fe.currentIndex)
			case FaultTypeBlocked:
				time.Sleep(fm.duration)
				err = fe.send(fm.m, fe.currentIndex)
			case FaultTypeDropped:
				err = fe.send(nil, fe.currentIndex)
			case FaultTypeDelayed:
				fe.sendQueue.push(&msgItem{
					sendAt: time.Now().Add(fm.duration),
					m:      fm.m,
					index:  fe.currentIndex,
				})
			default:
				fe.logger.Panic("unknown fault", zap.Any("fault", fault))
			}

			fe.currentIndex++

			fe.recordFault(fm.m.Type, fault)
			if isDup {
				fe.recordFault(fm.m.Type, FaultTypeDuplicated)
			}

			fe.errorc <- err

		case <-notifyc:
			items := fe.sendQueue.popUntil(time.Now())
			for _, di := range items {
				fe.send(di.m, di.index)
				if di.index < fe.maxSentIndex {
					fe.recordFault(di.m.Type, FaultTypeReordered)
				}
			}
		case <-statTicker.C:
			fe.reportFaultStat()
		case pr := <-fe.pendc:
			pr.c <- pendingStatus{nMessages: fe.sendQueue.len()}
		case <-fe.stopc:
			timer.Stop()
			statTicker.Stop()
			return
		}
	}
}
