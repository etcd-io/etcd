package pkg

import "fmt"

type Node interface {
	position() int
}

type noder struct{}

func (noder) position() int { panic("unreachable") }

func Fn() {
	nodes := []Node{struct {
		noder
	}{}}
	fmt.Println(nodes)
}
