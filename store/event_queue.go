package store

type eventQueue struct {
	Events   []*Event
	Size     int
	Front    int
	Capacity int
}

func (eq *eventQueue) back() int {
	return (eq.Front + eq.Size - 1 + eq.Capacity) % eq.Capacity
}

func (eq *eventQueue) insert(e *Event) {
	index := (eq.back() + 1) % eq.Capacity

	eq.Events[index] = e

	if eq.Size == eq.Capacity { //dequeue
		eq.Front = (index + 1) % eq.Capacity
	} else {
		eq.Size++
	}

}
