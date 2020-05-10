package pkg

type t181025 struct{}

func (t181025) F() {}

// package-level variable after function declaration used to trigger a
// bug in unused.

var V181025 t181025
