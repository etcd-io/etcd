package pkg

import "io"

func fn1() (io.ReadCloser, error) {
	return nil, nil
}

type T struct {
	rc io.ReadCloser
}

func fn3() (T, error) {
	return T{}, nil
}

func fn2() {
	rc, err := fn1()
	defer rc.Close() // want `should check returned error before deferring rc\.Close`
	if err != nil {
		println()
	}

	rc, _ = fn1()
	defer rc.Close()

	rc, err = fn1()
	if err != nil {
		println()
	}
	defer rc.Close()

	t, err := fn3()
	defer t.rc.Close() // want `should check returned error before deferring t\.rc\.Close`
	if err != nil {
		println()
	}
}
