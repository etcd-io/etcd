package main

import "fmt"

func main() {
	type Fraction struct {
		Numerator, Denominator int
	}

	one, two := 1, 9999
	two = 2
	frac := Fraction{one, two}
	frac.Denominator = 4
	for 1 > 5 {
		frac.Numerator = 3
	}
	select {}
	switch one = 111; one {
	}
	fmt.Println(frac)
}

//<<<<<debug,1,1,1,1,showlive,fail
//<<<<<debug,5,1,5,1,showlive,pass
