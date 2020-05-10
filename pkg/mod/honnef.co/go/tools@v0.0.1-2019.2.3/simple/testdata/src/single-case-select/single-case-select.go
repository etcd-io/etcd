package pkg

func fn() {
	var ch chan int
	select { // want `should use a simple channel send`
	case <-ch:
	}
outer:
	for { // want `should use for range`
		select {
		case <-ch:
			break outer
		}
	}

	for { // want `should use for range`
		select {
		case x := <-ch:
			_ = x
		}
	}

	for {
		select { // want `should use a simple channel send`
		case ch <- 0:
		}
	}
}
