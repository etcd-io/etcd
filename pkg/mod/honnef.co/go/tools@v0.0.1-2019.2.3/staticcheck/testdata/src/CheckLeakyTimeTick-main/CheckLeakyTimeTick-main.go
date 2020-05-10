package main

import "time"

func fn2() {
	for range time.Tick(0) {
		println("")
		if true {
			break
		}
	}
}

func main() {
	_ = time.Tick(0)
}
