// Package pkg ...
package pkg

type T1 int

func (x T1) Fn1()    {} // want `methods on the same type should have the same receiver name`
func (y T1) Fn2()    {}
func (x T1) Fn3()    {}
func (T1) Fn4()      {}
func (_ T1) Fn5()    {}
func (self T1) Fn6() {}
