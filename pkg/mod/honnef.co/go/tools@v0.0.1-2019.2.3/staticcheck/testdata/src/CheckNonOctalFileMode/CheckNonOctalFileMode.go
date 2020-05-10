package pkg

import "os"

func fn() {
	os.OpenFile("", 0, 644) // want `file mode.+`
}
