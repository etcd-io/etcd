package wait

import "sync"

type List struct {
	l sync.Mutex
	m map[int64]chan interface{}
}

func New() List {
	return List{m: make(map[int64]chan interface{})}
}

func (w List) Register(id int64) <-chan interface{} {
	w.l.Lock()
	defer w.l.Unlock()
	ch := w.m[id]
	if ch == nil {
		ch = make(chan interface{}, 1)
		w.m[id] = ch
	}
	return ch
}

func (w List) Trigger(id int64, x interface{}) {
	w.l.Lock()
	ch := w.m[id]
	delete(w.m, id)
	w.l.Unlock()
	if ch != nil {
		ch <- x
		close(ch)
	}
}
