package pkg

func fn() {
	var ch chan int
	for range ch {
		defer println() // want `defers in this range loop`
	}
}
