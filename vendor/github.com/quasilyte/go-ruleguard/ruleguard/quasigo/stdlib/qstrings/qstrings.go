package qstrings

import (
	"strings"

	"github.com/quasilyte/go-ruleguard/ruleguard/quasigo"
)

func ImportAll(env *quasigo.Env) {
	env.AddNativeFunc(`strings`, `Replace`, Replace)
	env.AddNativeFunc(`strings`, `ReplaceAll`, ReplaceAll)
	env.AddNativeFunc(`strings`, `TrimPrefix`, TrimPrefix)
	env.AddNativeFunc(`strings`, `TrimSuffix`, TrimSuffix)
	env.AddNativeFunc(`strings`, `HasPrefix`, HasPrefix)
	env.AddNativeFunc(`strings`, `HasSuffix`, HasSuffix)
	env.AddNativeFunc(`strings`, `Contains`, Contains)
}

func Replace(stack *quasigo.ValueStack) {
	n := stack.PopInt()
	newPart := stack.Pop().(string)
	oldPart := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.Replace(s, oldPart, newPart, n))
}

func ReplaceAll(stack *quasigo.ValueStack) {
	newPart := stack.Pop().(string)
	oldPart := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.ReplaceAll(s, oldPart, newPart))
}

func TrimPrefix(stack *quasigo.ValueStack) {
	prefix := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.TrimPrefix(s, prefix))
}

func TrimSuffix(stack *quasigo.ValueStack) {
	prefix := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.TrimSuffix(s, prefix))
}

func HasPrefix(stack *quasigo.ValueStack) {
	prefix := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.HasPrefix(s, prefix))
}

func HasSuffix(stack *quasigo.ValueStack) {
	suffix := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.HasSuffix(s, suffix))
}

func Contains(stack *quasigo.ValueStack) {
	substr := stack.Pop().(string)
	s := stack.Pop().(string)
	stack.Push(strings.Contains(s, substr))
}
