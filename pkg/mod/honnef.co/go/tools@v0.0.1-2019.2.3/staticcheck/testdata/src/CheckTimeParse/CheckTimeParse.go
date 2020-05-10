package pkg

import "time"

const c1 = "12345"
const c2 = "2006"

func fn() {
	time.Parse("12345", "") // want `parsing time`
	time.Parse(c1, "")      // want `parsing time`
	time.Parse(c2, "")
	time.Parse(time.RFC3339Nano, "")
	time.Parse(time.Kitchen, "")
}
