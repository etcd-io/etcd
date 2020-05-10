package pkg

func fn() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; i++ { // want `variable in loop condition never changes`
		}
	}
}
