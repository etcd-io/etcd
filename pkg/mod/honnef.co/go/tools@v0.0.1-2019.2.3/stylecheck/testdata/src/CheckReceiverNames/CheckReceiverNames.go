// Package pkg ...
package pkg

type T1 int

func (x T1) Fn1()    {}
func (y T1) Fn2()    {}
func (x T1) Fn3()    {}
func (T1) Fn4()      {}
func (_ T1) Fn5()    {} // want `receiver name should not be an underscore, omit the name if it is unused`
func (self T1) Fn6() {} // want `receiver name should be a reflection of its identity`
