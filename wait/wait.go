package wait

import "sync"

type Waiter struct {
	l sync.Mutex
	m map[int64]chan interface{}
}

func New() Waiter {
	return Waiter{m: make(map[int64]chan interface{})}
}

func (w Waiter) Register(id int64) <-chan interface{} {
	w.l.Lock()
	defer w.l.Unlock()
	ch := w.m[id]
	if ch == nil {
		ch = make(chan interface{}, 1)
		w.m[id] = ch
	}
	return ch
}

func (w Waiter) Trigger(id int64, x interface{}) {
	w.l.Lock()
	ch := w.m[id]
	delete(w.m, id)
	w.l.Unlock()
	if ch != nil {
		ch <- x
		close(ch)
	}
}
