package main

type I interface { String() string }
type S struct {}
func (s *S) String() string { return "" } //<<<<<debug,5,13,5,18,showreferences,pass

func String() { return "" }

func foo() string { return (&S{}).String() }
