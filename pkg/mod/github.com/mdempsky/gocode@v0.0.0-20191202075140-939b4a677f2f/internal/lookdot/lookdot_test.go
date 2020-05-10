package lookdot_test

import (
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"sort"
	"testing"

	"github.com/mdempsky/gocode/internal/lookdot"
)

const src = `
package p

import "time"

type S struct { x int; y int }
func (S) Sv()
func (*S) Sp()
var s S

type Q struct { Z }
var q Q

type I interface { f(); g() }

type P *S

type T1 struct { *T2 }
type T2 struct { *T1 }
func (*T1) t1()
func (*T2) t2()

type X int
func (*X) x()
type X1 struct { X }
type X2 struct { *X }
type X12 struct { X1; X2 }

type A1 int
func (A1) A() int
type A2 int
func (A2) A() int
type A struct { A1; A2; }

type B1 int
func (B1) b()
type B2 struct { b int; B1 }

var loc time.Location
`

var tests = []struct {
	lhs  string
	want []string
}{
	{"S", []string{"Sv"}},
	{"*S", []string{"Sv", "Sp"}},
	{"S{}", []string{"Sv", "x", "y"}},
	{"s", []string{"Sv", "Sp", "x", "y"}},

	{"I", []string{"f", "g"}},
	{"I(nil)", []string{"f", "g"}},
	{"(*I)(nil)", nil},

	// golang.org/issue/15708
	{"*T1", []string{"t1", "t2"}},
	{"T1", []string{"t2"}},

	// golang.org/issue/9060
	{"error", []string{"Error"}},
	{"struct { error }", []string{"Error"}},
	{"interface { error }", []string{"Error"}},

	// golang.org/issue/15722
	{"P", nil},

	{"X1", nil},
	{"X2", []string{"x"}},
	{"X12", nil},

	{"A", nil},

	{"B2", nil},

	{"loc", []string{"String"}},
}

func TestWalk(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "src.go", src, 0)
	if file == nil {
		t.Fatal(err)
	}

	cfg := types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	pkg, err := cfg.Check("p", fset, []*ast.File{file}, nil)
	if pkg == nil {
		t.Fatal(err)
	}

	// Add a test case for Go 1.11.
	if contains(build.Default.ReleaseTags, "go1.11") {
		tests = append(tests, struct {
			lhs  string
			want []string
		}{"q", []string{"Z"}})
	}

	for _, test := range tests {
		tv, err := types.Eval(fset, pkg, token.NoPos, test.lhs)
		if err != nil {
			t.Errorf("Eval(%q) failed: %v", test.lhs, err)
			continue
		}

		var got []string
		visitor := func(obj types.Object) {
			// TODO(mdempsky): Should Walk be responsible
			// for filtering out inaccessible objects?
			if obj.Exported() || obj.Pkg() == pkg {
				got = append(got, obj.Name())
			}
		}

		if !lookdot.Walk(&tv, visitor) {
			t.Errorf("Walk(%q) returned false", test.lhs)
			continue
		}

		sort.Strings(got)
		sort.Strings(test.want)

		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("Look(%q): got %v, want %v", test.lhs, got, test.want)
			continue
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
