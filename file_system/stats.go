package fileSystem

import (
	"encoding/json"
	"sync"
)

const (
	// Operations that will be running serializely
	StatsSetsHit         = 100
	StatsSetsMiss        = 101
	StatsDeletesHit      = 102
	StatsDeletesMiss     = 103
	StatsUpdatesHit      = 104
	StatsUpdatesMiss     = 105
	StatsTestAndSetsHit  = 106
	StatsTestAndSetsMiss = 107
	StatsRecoveryHit     = 108
	StatsRecoveryMiss    = 109

	// concurrent operations
	StatsGetsHit  = 200
	StatsGetsMiss = 201

	StatsWatchHit      = 300
	StatsWatchMiss     = 301
	StatsInWatchingNum = 302

	StatsSaveHit  = 400
	StatsSaveMiss = 401
)

type EtcdStats struct {

	// Lock for synchronization
	rwlock sync.RWMutex

	// Number of get requests
	GetsHit  uint64 `json:"gets_hits"`
	GetsMiss uint64 `json:"gets_misses"`

	// Number of sets requests
	SetsHit  uint64 `json:"sets_hits"`
	SetsMiss uint64 `json:"sets_misses"`

	// Number of delete requests
	DeletesHit  uint64 `json:"deletes_hits"`
	DeletesMiss uint64 `json:"deletes_misses"`

	// Number of update requests
	UpdatesHit  uint64 `json:"updates_hits"`
	UpdatesMiss uint64 `json:"updates_misses"`

	// Number of testAndSet requests
	TestAndSetsHit  uint64 `json:"testAndSets_hits"`
	TestAndSetsMiss uint64 `json:"testAndSets_misses"`

	// Number of Watch requests
	WatchHit      uint64 `json:"watch_hit"`
	WatchMiss     uint64 `json:"watch_miss"`
	InWatchingNum uint64 `json:"in_watching_number"`

	// Number of save requests
	SaveHit  uint64 `json:"save_hit"`
	SaveMiss uint64 `json:"save_miss"`

	// Number of recovery requests
	RecoveryHit  uint64 `json:"recovery_hit"`
	RecoveryMiss uint64 `json:"recovery_miss"`
}

func newStats() *EtcdStats {
	e := new(EtcdStats)
	return e
}

// Status() return the statistics info of etcd storage its recent start
func (e *EtcdStats) GetStats() []byte {
	b, _ := json.Marshal(e)
	return b
}

func (e *EtcdStats) TotalReads() uint64 {
	return e.GetsHit + e.GetsMiss
}

func (e *EtcdStats) TotalWrites() uint64 {
	return e.SetsHit + e.SetsMiss +
		e.DeletesHit + e.DeletesMiss +
		e.UpdatesHit + e.UpdatesMiss +
		e.TestAndSetsHit + e.TestAndSetsMiss
}

func (e *EtcdStats) IncStats(field int) {
	if field >= 200 {
		e.rwlock.Lock()

		switch field {
		case StatsGetsHit:
			e.GetsHit++
		case StatsGetsMiss:
			e.GetsMiss++
		case StatsWatchHit:
			e.WatchHit++
		case StatsWatchMiss:
			e.WatchMiss++
		case StatsInWatchingNum:
			e.InWatchingNum++
		case StatsSaveHit:
			e.SaveHit++
		case StatsSaveMiss:
			e.SaveMiss++
		}

		e.rwlock.Unlock()

	} else {
		e.rwlock.RLock()

		switch field {
		case StatsSetsHit:
			e.SetsHit++
		case StatsSetsMiss:
			e.SetsMiss++
		case StatsDeletesHit:
			e.DeletesHit++
		case StatsDeletesMiss:
			e.DeletesMiss++
		case StatsUpdatesHit:
			e.UpdatesHit++
		case StatsUpdatesMiss:
			e.UpdatesMiss++
		case StatsTestAndSetsHit:
			e.TestAndSetsHit++
		case StatsTestAndSetsMiss:
			e.TestAndSetsMiss++
		case StatsRecoveryHit:
			e.RecoveryHit++
		case StatsRecoveryMiss:
			e.RecoveryMiss++
		}

		e.rwlock.RUnlock()
	}

}
