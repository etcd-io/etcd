package pkg

func fn() {
	var ch chan int
	select {
	case <-ch:
	default:
	}

	for {
		select {
		case <-ch:
		default: // want `should not have an empty default case`
		}
	}

	for {
		select {
		case <-ch:
		default:
			println("foo")
		}
	}

	for {
		select {
		case <-ch:
		}
	}
}
