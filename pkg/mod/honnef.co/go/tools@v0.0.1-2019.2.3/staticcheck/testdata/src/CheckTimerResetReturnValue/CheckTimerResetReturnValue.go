package fn

import "time"

func fn1() {
	t := time.NewTimer(time.Second)
	t.Reset(time.Second)
}

func fn2() {
	t := time.NewTimer(time.Second)
	_ = t.Reset(time.Second)
}

func fn3() {
	t := time.NewTimer(time.Second)
	println(t.Reset(time.Second))
}

func fn4() {
	t := time.NewTimer(time.Second)
	if t.Reset(time.Second) {
		println("x")
	}
}

func fn5() {
	t := time.NewTimer(time.Second)
	if t.Reset(time.Second) { // want `it is not possible to use Reset's return value correctly`
		<-t.C
	}
}

func fn6(x bool) {
	// Not matched because we don't support complex boolean
	// expressions
	t := time.NewTimer(time.Second)
	if t.Reset(time.Second) || x {
		<-t.C
	}
}

func fn7(x bool) {
	// Not matched because we don't analyze that deeply
	t := time.NewTimer(time.Second)
	y := t.Reset(2 * time.Second)
	z := x || y
	println(z)
	if z {
		<-t.C
	}
}

func fn8() {
	t := time.NewTimer(time.Second)
	abc := t.Reset(time.Second) // want `it is not possible to use Reset's return value correctly`
	if abc {
		<-t.C
	}
}

func fn9() {
	t := time.NewTimer(time.Second)
	if t.Reset(time.Second) {
		println("x")
	}
	<-t.C
}

func fn10() {
	t := time.NewTimer(time.Second)
	if !t.Reset(time.Second) { // want `it is not possible to use Reset's return value correctly`
		<-t.C
	}
}

func fn11(ch chan int) {
	t := time.NewTimer(time.Second)
	if !t.Reset(time.Second) {
		<-ch
	}
}
