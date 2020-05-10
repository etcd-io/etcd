package main

type t1 struct {
	F1 int
}

type T2 struct {
	F2 int
}

func init() {
	_ = t1{}
	_ = T2{}
}
