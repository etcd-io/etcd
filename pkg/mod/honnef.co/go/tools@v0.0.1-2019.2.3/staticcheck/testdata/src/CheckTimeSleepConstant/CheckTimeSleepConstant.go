package pkg

import "time"

const c1 = 1
const c2 = 200

func fn() {
	time.Sleep(1)  // want `sleeping for 1`
	time.Sleep(42) // want `sleeping for 42`
	time.Sleep(201)
	time.Sleep(c1)
	time.Sleep(c2)
	time.Sleep(2 * time.Nanosecond)
	time.Sleep(time.Nanosecond)
}
