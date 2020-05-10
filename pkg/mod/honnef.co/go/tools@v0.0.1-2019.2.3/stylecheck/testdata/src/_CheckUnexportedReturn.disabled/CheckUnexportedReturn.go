// Package pkg ...
package pkg

type t1 int
type T2 int
type Recv int
type recv int

func fn1() string { return "" }
func Fn2() error  { return nil }
func fn3() error  { return nil }
func fn5() t1     { return 0 }
func Fn6() t1     { return 0 }   // want `should not return unexported type`
func Fn7() *t1    { return nil } // want `should not return unexported type`
func Fn8() T2     { return 0 }

func (Recv) fn9() t1  { return 0 }
func (Recv) Fn10() t1 { return 0 } // want `should not return unexported type`
func (Recv) Fn11() T2 { return 0 }

func (recv) fn9() t1  { return 0 }
func (recv) Fn10() t1 { return 0 }
func (recv) Fn11() T2 { return 0 }
