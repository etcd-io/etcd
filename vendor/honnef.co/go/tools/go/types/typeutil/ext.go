package typeutil

import (
	"fmt"
	"go/types"
)

type Iterator struct {
	elem types.Type
}

func (t *Iterator) Underlying() types.Type { return t }
func (t *Iterator) String() string         { return fmt.Sprintf("iterator(%s)", t.elem) }
func (t *Iterator) Elem() types.Type       { return t.elem }

func NewIterator(elem types.Type) *Iterator {
	return &Iterator{elem: elem}
}

type DeferStack struct{}

func (t *DeferStack) Underlying() types.Type { return t }
func (t *DeferStack) String() string         { return "deferStack" }

func NewDeferStack() *DeferStack {
	return &DeferStack{}
}
