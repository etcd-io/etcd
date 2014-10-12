package wait

import (
	"sync"
)

type Wait interface {
	Register(id uint64) <-chan interface{}
	Trigger(id uint64, x interface{})
}

type List struct {
	l sync.Mutex
	m map[uint64]chan interface{}
}

func New() *List {
	return &List{m: make(map[uint64]chan interface{})}
}

func (w *List) Register(id uint64) <-chan interface{} {
	w.l.Lock()
	defer w.l.Unlock()
	ch := w.m[id]
	if ch == nil {
		ch = make(chan interface{}, 1)
		w.m[id] = ch
	}
	return ch
}

func (w *List) Trigger(id uint64, x interface{}) {
	w.l.Lock()
	ch := w.m[id]
	delete(w.m, id)
	w.l.Unlock()
	if ch != nil {
		ch <- x
		close(ch)
	}
}
