package pkg

import (
	"sync"
)

func fn() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
	}()

	go func() {
		wg.Add(1) // want `should call wg\.Add\(1\) before starting`
		wg.Done()
	}()

	wg.Add(1)
	go func(wg sync.WaitGroup) {
		wg.Done()
	}(wg)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		wg.Done()
	}(&wg)

	wg.Wait()
}

func fn2(wg sync.WaitGroup) {
	wg.Add(1)
}
