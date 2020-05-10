// Copyright (c) 2017 David R. Jenni. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestFill(t *testing.T) {
	tests := [...]struct {
		name string
		src  string
		want string
	}{
		{
			name: "only basic types",
			src: `package p

import "unsafe"

var s = myStruct{}

type myStruct struct {
	a int
	b bool
	c complex64
	d uint16
	f float32
	g string
	h uintptr
	i unsafe.Pointer
}`,
			want: `myStruct{
	a: 0,
	b: false,
	c: (0 + 0i),
	d: 0,
	f: 0.0,
	g: "",
	h: uintptr(0),
	i: unsafe.Pointer(uintptr(0)),
}`,
		},
		{
			name: "make",
			src: `package p

import "io"

var s = myStruct{}

type myStruct struct {
	a chan int
	b chan myStruct
	c chan interface{}
	d chan io.Writer
}`,
			want: `myStruct{
	a: make(chan int),
	b: make(chan myStruct),
	c: make(chan interface{}),
	d: make(chan io.Writer),
}`,
		},
		{
			name: "basic composite types",
			src: `package p

import "io"

var s = myStruct{}

type myStruct struct {
	a *int
	c interface{}
	d io.Writer
	f func(int) bool
	g []int
}`,
			want: `myStruct{
	a: nil,
	c: nil,
	d: nil,
	f: func(int) bool { panic("not implemented") },
	g: nil,
}`,
		},
		{
			name: "pointer to struct",
			src: `package p

import (
	"container/list"
	"io"
)

var s = myStruct{}

type myStruct struct {
	a *otherStruct
	b otherStruct
	c [1]*otherStruct
	d **otherStruct
}

type otherStruct struct {
	a list.Element
	b *list.Element
	io.Reader
}

type anotherStruct struct{ a int }`,
			want: `myStruct{
	a: &otherStruct{
		a: list.Element{
			Value: nil,
		},
		b: &list.Element{
			Value: nil,
		},
		Reader: nil,
	},
	b: otherStruct{
		a: list.Element{
			Value: nil,
		},
		b: &list.Element{
			Value: nil,
		},
		Reader: nil,
	},
	c: [1]*otherStruct{
		{
			a: list.Element{
				Value: nil,
			},
			b: &list.Element{
				Value: nil,
			},
			Reader: nil,
		},
	},
	d: nil,
}`,
		},
		{
			name: "simple named types",
			src: `package p

import "io"

var s = myStruct{}

type (
	integer  int64
	reader   io.Reader
	slice    []integer
	functype func(reader) func(int) bool
)

type myStruct struct {
	a integer
	b reader
	c slice
	f functype
}`,
			want: `myStruct{
	a: 0,
	b: nil,
	c: nil,
	f: func(reader) func(int) bool { panic("not implemented") },
}`,
		},
		{
			name: "arrays",
			src: `package p

import "io"

var s = myStruct{}

type myStruct struct {
	a [3]float32
	b [3]io.Reader
	c interface{}
	d [0]int
	e [2][2]int
}`,
			want: `myStruct{
	a: [3]float32{
		0.0,
		0.0,
		0.0,
	},
	b: [3]io.Reader{
		nil,
		nil,
		nil,
	},
	c: nil,
	d: [0]int{},
	e: [2][2]int{
		{
			0,
			0,
		},
		{
			0,
			0,
		},
	},
}`,
		},
		{
			name: "advanced arrays",
			src: `package p

import (
	"io"
	"unsafe"
)

var s = myStruct{}

type myStruct struct {
	a [3]chan struct{}
	b [3]chan<- struct{}
	c [3]<-chan struct{}
	d [3]func(struct{}, interface{}) bool
	e [2][2][]unsafe.Pointer
	f [1]interface{io.Reader; foo(unsafe.Pointer, ...int) (bool, int) }
	g [1]chan (<-chan struct{})
	h [1]interface{ foo(x unsafe.Pointer, args ...string) }
	i [1]map[string]int
}`,
			want: `myStruct{
	a: [3]chan struct{}{
		make(chan struct{}),
		make(chan struct{}),
		make(chan struct{}),
	},
	b: [3]chan<- struct{}{
		make(chan<- struct{}),
		make(chan<- struct{}),
		make(chan<- struct{}),
	},
	c: [3]<-chan struct{}{
		make(<-chan struct{}),
		make(<-chan struct{}),
		make(<-chan struct{}),
	},
	d: [3]func(struct{}, interface{}) bool{
		func(struct{}, interface{}) bool { panic("not implemented") },
		func(struct{}, interface{}) bool { panic("not implemented") },
		func(struct{}, interface{}) bool { panic("not implemented") },
	},
	e: [2][2][]unsafe.Pointer{
		{
			nil,
			nil,
		},
		{
			nil,
			nil,
		},
	},
	f: [1]interface{Read(p []byte) (n int, err error); foo(unsafe.Pointer, ...int) (bool, int); io.Reader}{
		nil,
	},
	g: [1]chan (<-chan struct{}){
		make(chan <-chan struct{}),
	},
	h: [1]interface{foo(x unsafe.Pointer, args ...string)}{
		nil,
	},
	i: [1]map[string]int{
		map[string]int{
			"": 0,
		},
	},
}`,
		},
		{
			name: "nested structs",
			src: `package p

import "container/list"

var s = myStruct{}

type myStruct struct {
	a [3]struct{a int; b int}
	b list.Element
	c [2]otherStruct
	d struct{c *list.Element; D complex64; *list.Element}
	e [2]list.Element
}

type otherStruct struct {
	a bool
	B int
}`,
			want: `myStruct{
	a: [3]struct{a int; b int}{
		{
			a: 0,
			b: 0,
		},
		{
			a: 0,
			b: 0,
		},
		{
			a: 0,
			b: 0,
		},
	},
	b: list.Element{
		Value: nil,
	},
	c: [2]otherStruct{
		{
			a: false,
			B: 0,
		},
		{
			a: false,
			B: 0,
		},
	},
	d: struct{c *list.Element; D complex64; *list.Element}{
		c: &list.Element{
			Value: nil,
		},
		D: (0 + 0i),
		Element: &list.Element{
			Value: nil,
		},
	},
	e: [2]list.Element{
		{
			Value: nil,
		},
		{
			Value: nil,
		},
	},
}`,
		},
		{
			name: "with existing key-value exprs",
			src: `package p

import "container/list"

var s = myStruct{
	a: 42,
	c: &otherStruct{
		a: 1,
		b: 3,
	},
	d: "foo",
	e: otherStruct{b: 9},
	f: [1]otherStruct{{c: 42}},
}

type myStruct struct {
	a int
	b *list.Element
	c *otherStruct 
	d string
	e otherStruct
	f [1]otherStruct
}

type otherStruct struct{ a, b, c int}`,
			want: `myStruct{
	a: 42,
	b: &list.Element{
		Value: nil,
	},
	c: &otherStruct{
		a: 1,
		b: 3,
	},
	d: "foo",
	e: otherStruct{
		b: 9,
	},
	f: [1]otherStruct{
		{
			c: 42,
		},
	},
}`,
		},
		{
			name: "real-world example",
			src: `package p

import (
	"go/ast"
	"go/types"
	"go/token"
)

var s = myStruct{}

type myStruct struct {
	pkg  *types.Package
	fset *token.FileSet
	lit  *ast.CallExpr
	typ  *types.Struct
	name *types.Named
}`,
			want: `myStruct{
	pkg:  &types.Package{},
	fset: &token.FileSet{},
	lit: &ast.CallExpr{
		Fun:      nil,
		Lparen:   0,
		Args:     nil,
		Ellipsis: 0,
		Rparen:   0,
	},
	typ:  &types.Struct{},
	name: &types.Named{},
}`,
		},
		{
			name: "maps",
			src: `package p

import "io"

var s = myStruct{}

type duration uint

type myStruct struct {
	a map[string]int
	b map[io.Reader]string
	c map[*io.Reader]map[int]string
	d map[int]*io.LimitedReader
	e map[string]duration
}`,
			want: `myStruct{
	a: map[string]int{
		"": 0,
	},
	b: map[io.Reader]string{
		nil: "",
	},
	c: map[*io.Reader]map[int]string{
		nil: map[int]string{
			0: "",
		},
	},
	d: map[int]*io.LimitedReader{
		0: {
			R: nil,
			N: 0,
		},
	},
	e: map[string]duration{
		"": 0,
	},
}`,
		},
		{
			name: "type errors",
			src: `package p

import "io"

var s = myStruct{}

var x int = "type error"

type myStruct struct {
	a int
	b in // type error
	c int
	d io.Reade // type error
	e io.Reader
	f []int
	g [1]struct{
		x boo  // type error
	}
	h [1]strin // type error
}`,
			want: `myStruct{
	a: 0,
	c: 0,
	e: nil,
	f: nil,
}`,
		},
		/*
			TODO: This test breaks under Go 1.11.
			{
				name: "recursive struct definitions",
				src: `package p

				import "unsafe"

				var s = myStruct{}

				type myStruct struct {
					a *myStruct
					b [1]*myStruct
					c *otherStruct
					z myStruct // type error: invalid recursive type myStruct
				}

				type otherStruct struct {
					a *myStruct
				}
				`,
				want: `myStruct{
					a: &myStruct{},
					b: [1]*myStruct{
						{},
					},
					c: &otherStruct{
						a: &myStruct{},
					},
				}`,
			},
		*/
		{
			name: "renamed imports",
			src: `package p

import (
	goast "go/ast"
	. "io"
	_ "io"
)

var s = myStruct{}

type myStruct struct {
	a *goast.Ident
	b LimitedReader
}
`,
			want: `myStruct{
	a: &goast.Ident{
		NamePos: 0,
		Name:    "",
		Obj: &goast.Object{
			Kind: 0,
			Name: "",
			Decl: nil,
			Data: nil,
			Type: nil,
		},
	},
	b: LimitedReader{
		R: nil,
		N: 0,
	},
}`,
		}, {
			name: "gRPC types",
			src: `package p

import "unsafe"

var s = myStruct{}

type myStruct struct {
	Name                 string
	XXX_NoUnkeyedLiteral struct{}
	XXX_unrecognized     []byte
	XXX_sizecache        int32
	XXX_whatever         string
}`,
			want: `myStruct{
	Name: "",
}`,
		},
	}

	for _, test := range tests {
		pkg, importNames, lit, typ := parseStruct(t, test.name, test.src)

		name := types.NewNamed(types.NewTypeName(0, pkg, "myStruct", nil), typ, nil)
		newlit, lines := zeroValue(pkg, importNames, lit, litInfo{typ: typ, name: name})

		out := printNode(t, test.name, newlit, lines)
		if test.want != out {
			t.Errorf("%q: got %v, want %v\n", test.name, out, test.want)
		}
	}
}

func parseStruct(t *testing.T, filename, src string) (*types.Package, map[string]string, *ast.CompositeLit, *types.Struct) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	info := types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	conf := types.Config{
		Importer: importer.Default(),
		Error:    func(err error) {},
	}

	pkg, _ := conf.Check(f.Name.Name, fset, []*ast.File{f}, &info)
	importNames := buildImportNameMap(f)

	expr := f.Decls[1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]
	return pkg, importNames, expr.(*ast.CompositeLit), info.Types[expr].Type.Underlying().(*types.Struct)
}

func printNode(t *testing.T, name string, n ast.Node, lines int) string {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, lines)
	for i := 1; i <= lines; i++ {
		file.AddLine(i)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, n); err != nil {
		t.Fatalf("%q: %v", name, err)
	}
	return buf.String()
}
