package pkg

func fn() {
	var b1, b2 []byte
	for i, v := range b1 { // want `should use copy`
		b2[i] = v
	}

	for i := range b1 { // want `should use copy`
		b2[i] = b1[i]
	}

	type T [][16]byte
	var a T
	b := make([]interface{}, len(a))
	for i := range b {
		b[i] = a[i]
	}

	var b3, b4 []*byte
	for i := range b3 { // want `should use copy`
		b4[i] = b3[i]
	}

	var m map[int]byte
	for i, v := range b1 {
		m[i] = v
	}

}
