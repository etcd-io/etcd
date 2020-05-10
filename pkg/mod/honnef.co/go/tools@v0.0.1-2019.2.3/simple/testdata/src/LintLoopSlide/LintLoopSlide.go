package pkg

func fn() {
	var n int
	var bs []int
	var offset int

	for i := 0; i < n; i++ { // want `should use copy\(bs\[:n\], bs\[offset:\]\) instead`
		bs[i] = bs[offset+i]
	}

	for i := 1; i < n; i++ { // not currently supported
		bs[i] = bs[offset+i]
	}

	for i := 1; i < n; i++ { // not currently supported
		bs[i] = bs[i+offset]
	}

	for i := 0; i <= n; i++ {
		bs[i] = bs[offset+i]
	}
}
