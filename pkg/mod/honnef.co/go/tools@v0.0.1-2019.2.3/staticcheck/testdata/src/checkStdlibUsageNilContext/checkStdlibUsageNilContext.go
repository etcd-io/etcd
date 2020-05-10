package pkg

import "context"

func fn1(ctx context.Context)           {}
func fn2(x string, ctx context.Context) {}

func fn3() {
	fn1(nil) // want `do not pass a nil Context`
	fn1(context.TODO())
	fn2("", nil)
}
