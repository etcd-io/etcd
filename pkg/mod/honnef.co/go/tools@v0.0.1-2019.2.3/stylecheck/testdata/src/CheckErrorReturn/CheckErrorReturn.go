// Package pkg ...
package pkg

func fn1() (error, int)        { return nil, 0 }      // want `error should be returned as the last argument`
func fn2() (a, b error, c int) { return nil, nil, 0 } // want `error should be returned as the last argument`
func fn3() (a int, b, c error) { return 0, nil, nil }
func fn4() (error, error)      { return nil, nil }
func fn5() int                 { return 0 }
func fn6() (int, error)        { return 0, nil }
func fn7() (error, int, error) { return nil, 0, nil }
