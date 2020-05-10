package pkg

import "time"

func fn1() {
	for range time.Tick(0) {
		println("")
	}
}

func fn2() {
	for range time.Tick(0) { // want `leaks the underlying ticker`
		println("")
		if true {
			break
		}
	}
}

func fn3() {
	for range time.Tick(0) { // want `leaks the underlying ticker`
		println("")
		if true {
			return
		}
	}
}

func fn4() {
	go func() {
		for range time.Tick(0) {
			println("")
		}
	}()
}

type T struct{}

func (t *T) foo() {
	for range time.Tick(0) {
		println("")
	}
}

func (t T) bar() {
	for range time.Tick(0) {
		println("")
	}
}
