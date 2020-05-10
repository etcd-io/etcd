package pkg

//go:cgo_export_dynamic
func foo() {}

func bar() {} // want `bar`
