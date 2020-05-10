package pkg

import "time"

func fn() {
	t1 := time.Now()
	_ = time.Now().Sub(t1) // want `time\.Since`
	_ = time.Date(0, 0, 0, 0, 0, 0, 0, nil).Sub(t1)
}
