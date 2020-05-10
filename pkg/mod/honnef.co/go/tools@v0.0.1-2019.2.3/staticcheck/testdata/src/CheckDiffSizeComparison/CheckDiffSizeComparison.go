package pkg

func fn1() {
	var s1 = "foobar"
	_ = "a"[:] == s1           // want `comparing strings of different sizes`
	_ = s1 == "a"[:]           // want `comparing strings of different sizes`
	_ = "a"[:] == s1[:2]       // want `comparing strings of different sizes`
	_ = "ab"[:] == s1[1:2]     // want `comparing strings of different sizes`
	_ = "ab"[:] == s1[0+1:2]   // want `comparing strings of different sizes`
	_ = "a"[:] == "abc"        // want `comparing strings of different sizes`
	_ = "a"[:] == "a"+"bc"     // want `comparing strings of different sizes`
	_ = "foobar"[:] == s1+"bc" // want `comparing strings of different sizes`
	_ = "a"[:] == "abc"[:]     // want `comparing strings of different sizes`
	_ = "a"[:] == "abc"[:2]    // want `comparing strings of different sizes`

	_ = "a" == s1 // ignores
	_ = s1 == "a" // ignored
	_ = "abcdef"[:] == s1
	_ = "ab"[:] == s1[:2]
	_ = "a"[:] == s1[1:2]
	_ = "a"[:] == s1[0+1:2]
	_ = "abc"[:] == "abc"
	_ = "abc"[:] == "a"+"bc"
	_ = s1[:] == "foo"+"bar"
	_ = "abc"[:] == "abc"[:]
	_ = "ab"[:] == "abc"[:2]
}

func fn2() {
	s1 := "123"
	if true {
		s1 = "1234"
	}

	_ = s1 == "12345"[:] // want `comparing strings of different sizes`
	_ = s1 == "1234"[:]
	_ = s1 == "123"[:]
	_ = s1 == "12"[:] // want `comparing strings of different sizes`
}

func fn3(x string) {
	switch x[:1] {
	case "a":
	case "ab": // want `comparing strings of different sizes`
	case "b":
	case "bc": // want `comparing strings of different sizes`
	}
}
