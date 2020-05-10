package testdata

import "go/types"

type Bar struct{}

type Bazzar interface {
	Baz() string
}

type baz struct{}

func (b *baz) Baz() string { return "" }

type Celsius float64

type Fahrenheit float64

type Person struct {
	FirstName string
	LastName  string
	Age       int
	Gender    string
	Siblings  []*Person
}

type Doctor struct {
	*Person
	ID          string
	numPatients int
	string
}

type name string

type Name struct {
	Name string
}

type Reserved struct {
	Name        string
	Break       string
	Default     string
	Func        string
	Interface   string
	Select      string
	Case        string
	Defer       string
	Go          string
	Map         string
	Struct      string
	Chan        string
	Else        string
	Goto        string
	Package     string
	Switch      string
	Const       string
	Fallthrough string
	If          string
	Range       string
	Type        string
	Continue    string
	For         string
	Import      string
	Return      string
	Var         string
}

type Importer struct {
	Importer types.Importer
	Field    *types.Var
}

type SameTypeName struct{}

type sameTypeName struct{}
