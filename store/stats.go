package store

import (
	"encoding/json"
	"sync/atomic"
)

const (
	SetSuccess            = 100
	SetFail               = 101
	DeleteSuccess         = 102
	DeleteFail            = 103
	UpdateSuccess         = 104
	UpdateFail            = 105
	CompareAndSwapSuccess = 106
	CompareAndSwapFail    = 107
	GetSuccess            = 110
	GetFail               = 111
	ExpireCount           = 112
)

type Stats struct {

	// Number of get requests
	GetSuccess uint64 `json:"getsSuccess"`
	GetFail    uint64 `json:"getsFail"`

	// Number of sets requests
	SetSuccess uint64 `json:"setsSuccess"`
	SetFail    uint64 `json:"setsFail"`

	// Number of delete requests
	DeleteSuccess uint64 `json:"deleteSuccess"`
	DeleteFail    uint64 `json:"deleteFail"`

	// Number of update requests
	UpdateSuccess uint64 `json:"updateSuccess"`
	UpdateFail    uint64 `json:"updateFail"`

	// Number of testAndSet requests
	CompareAndSwapSuccess uint64 `json:"compareAndSwapSuccess"`
	CompareAndSwapFail    uint64 `json:"compareAndSwapFail"`

	ExpireCount uint64 `json:"expireCount"`

	Watchers uint64 `json:"watchers"`
}

func newStats() *Stats {
	s := new(Stats)
	return s
}

func (s *Stats) clone() *Stats {
	return &Stats{s.GetSuccess, s.GetFail, s.SetSuccess, s.SetFail,
		s.DeleteSuccess, s.DeleteFail, s.UpdateSuccess, s.UpdateFail,
		s.CompareAndSwapSuccess, s.CompareAndSwapFail, s.Watchers, s.ExpireCount}
}

// Status() return the statistics info of etcd storage its recent start
func (s *Stats) toJson() []byte {
	b, _ := json.Marshal(s)
	return b
}

func (s *Stats) TotalReads() uint64 {
	return s.GetSuccess + s.GetFail
}

func (s *Stats) TotalWrites() uint64 {
	return s.SetSuccess + s.SetFail +
		s.DeleteSuccess + s.DeleteFail +
		s.CompareAndSwapSuccess + s.CompareAndSwapFail +
		s.UpdateSuccess + s.UpdateFail
}

func (s *Stats) Inc(field int) {
	switch field {
	case SetSuccess:
		atomic.AddUint64(&s.SetSuccess, 1)
	case SetFail:
		atomic.AddUint64(&s.SetFail, 1)
	case DeleteSuccess:
		atomic.AddUint64(&s.DeleteSuccess, 1)
	case DeleteFail:
		atomic.AddUint64(&s.DeleteFail, 1)
	case GetSuccess:
		atomic.AddUint64(&s.GetSuccess, 1)
	case GetFail:
		atomic.AddUint64(&s.GetFail, 1)
	case UpdateSuccess:
		atomic.AddUint64(&s.UpdateSuccess, 1)
	case UpdateFail:
		atomic.AddUint64(&s.UpdateFail, 1)
	case CompareAndSwapSuccess:
		atomic.AddUint64(&s.CompareAndSwapSuccess, 1)
	case CompareAndSwapFail:
		atomic.AddUint64(&s.CompareAndSwapFail, 1)
	case ExpireCount:
		atomic.AddUint64(&s.ExpireCount, 1)
	}
}
