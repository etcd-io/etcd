// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"hash/fnv"
	"strconv"
	"sync"

	pb "github.com/pingcap/tipb/go-binlog"
)

const (
	// Range means range algorithm.
	Range = "range"

	// Hash means hash algorithm.
	Hash = "hash"

	// Score means choose pump by it's score.
	Score = "score"

	// LocalUnix means will only use the local pump by unix socket.
	LocalUnix = "local unix"
)

// PumpSelector selects pump for sending binlog.
type PumpSelector interface {
	// SetPumps set pumps to be selected.
	SetPumps([]*PumpStatus)

	// Select returns a situable pump.
	Select(*pb.Binlog) *PumpStatus

	// returns the next pump.
	Next(*pb.Binlog, int) *PumpStatus
}

// HashSelector select a pump by hash.
type HashSelector struct {
	sync.RWMutex

	// TsMap saves the map of start_ts with pump when send prepare binlog.
	// And Commit binlog should send to the same pump.
	TsMap map[int64]*PumpStatus

	// PumpMap saves the map of pump's node id with pump.
	PumpMap map[string]*PumpStatus

	// the pumps to be selected.
	Pumps []*PumpStatus
}

// NewHashSelector returns a new HashSelector.
func NewHashSelector() PumpSelector {
	return &HashSelector{
		TsMap:   make(map[int64]*PumpStatus),
		PumpMap: make(map[string]*PumpStatus),
		Pumps:   make([]*PumpStatus, 0, 10),
	}
}

// SetPumps implement PumpSelector.SetPumps.
func (h *HashSelector) SetPumps(pumps []*PumpStatus) {
	h.Lock()
	h.PumpMap = make(map[string]*PumpStatus)
	h.Pumps = pumps
	for _, pump := range pumps {
		h.PumpMap[pump.NodeID] = pump
	}
	h.Unlock()
}

// Select implement PumpSelector.Select.
func (h *HashSelector) Select(binlog *pb.Binlog) *PumpStatus {
	// TODO: use status' label to match situale pump.
	h.Lock()
	defer h.Unlock()

	if pump, ok := h.TsMap[binlog.StartTs]; ok {
		// binlog is commit binlog or rollback binlog, choose the same pump by start ts map.
		delete(h.TsMap, binlog.StartTs)
		return pump
	}

	if len(h.Pumps) == 0 {
		return nil
	}

	if binlog.Tp == pb.BinlogType_Prewrite {
		pump := h.Pumps[hashTs(binlog.StartTs)%len(h.Pumps)]
		h.TsMap[binlog.StartTs] = pump
		return pump
	}

	// can't find pump in ts map, or unkow binlog type, choose a new one.
	return h.Pumps[hashTs(binlog.StartTs)%len(h.Pumps)]
}

// Next implement PumpSelector.Next. Only for Prewrite binlog.
func (h *HashSelector) Next(binlog *pb.Binlog, retryTime int) *PumpStatus {
	h.Lock()
	defer h.Unlock()

	if len(h.Pumps) == 0 {
		return nil
	}

	nextPump := h.Pumps[(hashTs(binlog.StartTs)+int(retryTime))%len(h.Pumps)]
	if binlog.Tp == pb.BinlogType_Prewrite {
		h.TsMap[binlog.StartTs] = nextPump
	}

	return nextPump
}

// RangeSelector select a pump by range.
type RangeSelector struct {
	sync.RWMutex

	// Offset saves the offset in Pumps.
	Offset int

	// TsMap saves the map of start_ts with pump when send prepare binlog.
	// And Commit binlog should send to the same pump.
	TsMap map[int64]*PumpStatus

	// PumpMap saves the map of pump's node id with pump.
	PumpMap map[string]*PumpStatus

	// the pumps to be selected.
	Pumps []*PumpStatus
}

// NewRangeSelector returns a new ScoreSelector.
func NewRangeSelector() PumpSelector {
	return &RangeSelector{
		Offset:  0,
		TsMap:   make(map[int64]*PumpStatus),
		PumpMap: make(map[string]*PumpStatus),
		Pumps:   make([]*PumpStatus, 0, 10),
	}
}

// SetPumps implement PumpSelector.SetPumps.
func (r *RangeSelector) SetPumps(pumps []*PumpStatus) {
	r.Lock()
	r.PumpMap = make(map[string]*PumpStatus)
	r.Pumps = pumps
	for _, pump := range pumps {
		r.PumpMap[pump.NodeID] = pump
	}
	r.Offset = 0
	r.Unlock()
}

// Select implement PumpSelector.Select.
func (r *RangeSelector) Select(binlog *pb.Binlog) *PumpStatus {
	// TODO: use status' label to match situale pump.
	r.Lock()
	defer func() {
		if r.Offset == len(r.Pumps) {
			r.Offset = 0
		}
		r.Unlock()
	}()

	if pump, ok := r.TsMap[binlog.StartTs]; ok {
		// binlog is commit binlog or rollback binlog, choose the same pump by start ts map.
		delete(r.TsMap, binlog.StartTs)
		return pump
	}

	if len(r.Pumps) == 0 {
		return nil
	}

	if r.Offset >= len(r.Pumps) {
		r.Offset = 0
	}

	if binlog.Tp == pb.BinlogType_Prewrite {
		pump := r.Pumps[r.Offset]
		r.TsMap[binlog.StartTs] = pump
		r.Offset++
		return pump
	}

	// can't find pump in ts map, or the pump is not avaliable, choose a new one.
	return r.Pumps[r.Offset]
}

// Next implement PumpSelector.Next. Only for Prewrite binlog.
func (r *RangeSelector) Next(binlog *pb.Binlog, retryTime int) *PumpStatus {
	r.Lock()
	defer func() {
		if len(r.Pumps) != 0 {
			r.Offset = (r.Offset + 1) % len(r.Pumps)
		}
		r.Unlock()
	}()

	if len(r.Pumps) == 0 {
		return nil
	}

	if r.Offset >= len(r.Pumps) {
		r.Offset = 0
	}

	nextPump := r.Pumps[r.Offset]
	if binlog.Tp == pb.BinlogType_Prewrite {
		r.TsMap[binlog.StartTs] = nextPump
	}

	return nextPump
}

// LocalUnixSelector will always select the local pump, used for compatible with kafka version tidb-binlog.
type LocalUnixSelector struct {
	sync.RWMutex

	// the pump to be selected.
	Pump *PumpStatus
}

// NewLocalUnixSelector returns a new LocalUnixSelector.
func NewLocalUnixSelector() PumpSelector {
	return &LocalUnixSelector{}
}

// SetPumps implement PumpSelector.SetPumps.
func (u *LocalUnixSelector) SetPumps(pumps []*PumpStatus) {
	u.Lock()
	if len(pumps) == 0 {
		u.Pump = nil
	} else {
		u.Pump = pumps[0]
	}
	u.Unlock()
}

// Select implement PumpSelector.Select.
func (u *LocalUnixSelector) Select(binlog *pb.Binlog) *PumpStatus {
	u.RLock()
	defer u.RUnlock()

	return u.Pump
}

// Next implement PumpSelector.Next. Only for Prewrite binlog.
func (u *LocalUnixSelector) Next(binlog *pb.Binlog, retryTime int) *PumpStatus {
	u.RLock()
	defer u.RUnlock()

	return u.Pump
}

// ScoreSelector select a pump by pump's score.
type ScoreSelector struct{}

// NewScoreSelector returns a new ScoreSelector.
func NewScoreSelector() PumpSelector {
	return &ScoreSelector{}
}

// SetPumps implement PumpSelector.SetPumps.
func (s *ScoreSelector) SetPumps(pumps []*PumpStatus) {
	// TODO
}

// Select implement PumpSelector.Select.
func (s *ScoreSelector) Select(binlog *pb.Binlog) *PumpStatus {
	// TODO
	return nil
}

// Next implement PumpSelector.Next. Only for Prewrite binlog.
func (s *ScoreSelector) Next(binlog *pb.Binlog, retryTime int) *PumpStatus {
	// TODO
	return nil
}

// NewSelector returns a PumpSelector according to the algorithm.
func NewSelector(algorithm string) PumpSelector {
	var selector PumpSelector
	switch algorithm {
	case Range:
		selector = NewRangeSelector()
	case Hash:
		selector = NewHashSelector()
	case Score:
		selector = NewScoreSelector()
	case LocalUnix:
		selector = NewLocalUnixSelector()
	default:
		Logger.Warnf("unknow algorithm %s, use range as default", algorithm)
		selector = NewRangeSelector()
	}

	return selector
}

func hashTs(ts int64) int {
	h := fnv.New32a()
	h.Write([]byte(strconv.FormatInt(ts, 10)))
	return int(h.Sum32())
}
