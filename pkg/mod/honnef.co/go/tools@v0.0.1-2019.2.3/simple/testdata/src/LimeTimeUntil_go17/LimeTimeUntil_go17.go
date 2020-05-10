package pkg

import "time"

func fn(t time.Time) {
	t.Sub(time.Now())
}
