package main

import "fmt"

func main() {
	n := 1
	if true {
		n = n + 1
		fmt.Println(n)
		if n == 17 {
			return
		}
	}
	for _, v := range []int{100, 1000} {
		n = n + v
		fmt.Println(n)
		if n > -3 {
			continue
		}
		if n < -100 {
			break
		}
	}
	switch n {
	case 0:
		fallthrough
	case 1:
		fmt.Println("Impossible")
	default:
		fmt.Println("OK")
	}
}

//<<<<<debug,1,1,1,1,showcfg,fail
//<<<<<debug,5,1,5,1,showcfg,pass
