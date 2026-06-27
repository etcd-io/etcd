package qstrconv

import (
	"strconv"

	"github.com/quasilyte/go-ruleguard/ruleguard/quasigo"
)

func ImportAll(env *quasigo.Env) {
	env.AddNativeFunc(`strconv`, `Atoi`, Atoi)
	env.AddNativeFunc(`strconv`, `Itoa`, Itoa)
}

func Atoi(stack *quasigo.ValueStack) {
	s := stack.Pop().(string)
	v, err := strconv.Atoi(s)
	stack.PushInt(v)
	stack.Push(err)
}

func Itoa(stack *quasigo.ValueStack) {
	i := stack.PopInt()
	stack.Push(strconv.Itoa(i))
}
