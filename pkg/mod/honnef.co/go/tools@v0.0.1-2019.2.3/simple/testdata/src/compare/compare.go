package pkg

import "bytes"

func fn() {
	_ = bytes.Compare(nil, nil) == 0 // want ` bytes.Equal`
	_ = bytes.Compare(nil, nil) != 0 // want `!bytes.Equal`
	_ = bytes.Compare(nil, nil) > 0
	_ = bytes.Compare(nil, nil) < 0
}
