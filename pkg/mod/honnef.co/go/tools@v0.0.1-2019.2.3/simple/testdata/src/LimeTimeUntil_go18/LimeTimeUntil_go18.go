package pkg

import "time"

func fn(t time.Time) {
	t.Sub(time.Now()) // want `time\.Until`
	t.Sub(t)
	t2 := time.Now()
	t.Sub(t2)
}
