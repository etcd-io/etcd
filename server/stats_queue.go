package server

import (
	"sync"
	"time"
)

const (
	queueCapacity = 200
)

type statsQueue struct {
	items        [queueCapacity]*packageStats
	size         int
	front        int
	back         int
	totalPkgSize int
	rwl          sync.RWMutex
}

func (q *statsQueue) Len() int {
	return q.size
}

func (q *statsQueue) PkgSize() int {
	return q.totalPkgSize
}

// FrontAndBack gets the front and back elements in the queue
// We must grab front and back together with the protection of the lock
func (q *statsQueue) frontAndBack() (*packageStats, *packageStats) {
	q.rwl.RLock()
	defer q.rwl.RUnlock()
	if q.size != 0 {
		return q.items[q.front], q.items[q.back]
	}
	return nil, nil
}

// Insert function insert a packageStats into the queue and update the records
func (q *statsQueue) Insert(p *packageStats) {
	q.rwl.Lock()
	defer q.rwl.Unlock()

	q.back = (q.back + 1) % queueCapacity

	if q.size == queueCapacity { //dequeue
		q.totalPkgSize -= q.items[q.front].size
		q.front = (q.back + 1) % queueCapacity
	} else {
		q.size++
	}

	q.items[q.back] = p
	q.totalPkgSize += q.items[q.back].size

}

// Rate function returns the package rate and byte rate
func (q *statsQueue) Rate() (float64, float64) {
	front, back := q.frontAndBack()

	if front == nil || back == nil {
		return 0, 0
	}

	if time.Now().Sub(back.Time()) > time.Second {
		q.Clear()
		return 0, 0
	}

	sampleDuration := back.Time().Sub(front.Time())

	pr := float64(q.Len()) / float64(sampleDuration) * float64(time.Second)

	br := float64(q.PkgSize()) / float64(sampleDuration) * float64(time.Second)

	return pr, br
}

// Clear function clear up the statsQueue
func (q *statsQueue) Clear() {
	q.rwl.Lock()
	defer q.rwl.Unlock()
	q.back = -1
	q.front = 0
	q.size = 0
	q.totalPkgSize = 0
}
