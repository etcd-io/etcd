package pkg

import "sync"

var r sync.Mutex
var rw sync.RWMutex

func fn1() {
	r.Lock()
	defer r.Lock() // want `deferring Lock right after having locked already; did you mean to defer Unlock`
}

func fn2() {
	r.Lock()
	defer r.Unlock()
}

func fn3() {
	println("")
	defer r.Lock()
}

func fn4() {
	rw.RLock()
	defer rw.RLock() // want `deferring RLock right after having locked already; did you mean to defer RUnlock`
}

func fn5() {
	rw.RLock()
	defer rw.Lock()
}

func fn6() {
	r.Lock()
	defer rw.Lock()
}
