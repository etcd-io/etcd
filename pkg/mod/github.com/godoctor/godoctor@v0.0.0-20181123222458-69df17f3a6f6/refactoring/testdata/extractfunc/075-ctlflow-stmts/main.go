package main

func main() {
	for i := 1; i <= 10; i++ {
		break //<<<<<extract,5,1,6,1,foo,fail
	}
	for i := 1; i <= 10; i++ {
		continue //<<<<<extract,8,1,9,1,foo,fail
	}
	goto lbl
lbl: //<<<<<extract,11,1,12,11,foo,fail
	n := 7 //<<<<<extract,12,6,12,11,foo,fail
	for i := 1; i <= n; i++ {
	}

	defer foo() //<<<<<extract,16,2,16,12,bar,fail
}

func foo() {}
