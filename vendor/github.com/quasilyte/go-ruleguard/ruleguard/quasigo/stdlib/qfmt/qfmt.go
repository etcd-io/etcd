package qfmt

import (
	"fmt"

	"github.com/quasilyte/go-ruleguard/ruleguard/quasigo"
)

func ImportAll(env *quasigo.Env) {
	env.AddNativeFunc(`fmt`, `Sprintf`, Sprintf)
}

func Sprintf(stack *quasigo.ValueStack) {
	args := stack.PopVariadic()
	format := stack.Pop().(string)
	stack.Push(fmt.Sprintf(format, args...))
}
