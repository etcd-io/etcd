// Package pkg is amazing.
package pkg

import (
	"fmt"
	"math/big"
	"os"
)

type Error int

func (Error) Error() string { return "" }

func fn() {
	var b bool
	var i int
	var r rune
	var s string
	var x float64
	var p *int
	var imap map[int]int
	var fslice []float64
	var c complex64
	// Some good format/argtypes
	fmt.Printf("")
	fmt.Printf("%b %b %b", 3, i, x)
	fmt.Printf("%c %c %c %c", 3, i, 'x', r)
	fmt.Printf("%d %d %d", 3, i, imap)
	fmt.Printf("%e %e %e %e", 3e9, x, fslice, c)
	fmt.Printf("%E %E %E %E", 3e9, x, fslice, c)
	fmt.Printf("%f %f %f %f", 3e9, x, fslice, c)
	fmt.Printf("%F %F %F %F", 3e9, x, fslice, c)
	fmt.Printf("%g %g %g %g", 3e9, x, fslice, c)
	fmt.Printf("%G %G %G %G", 3e9, x, fslice, c)
	fmt.Printf("%b %b %b %b", 3e9, x, fslice, c)
	fmt.Printf("%o %o", 3, i)
	fmt.Printf("%p", p)
	fmt.Printf("%q %q %q %q", 3, i, 'x', r)
	fmt.Printf("%s %s %s", "hi", s, []byte{65})
	fmt.Printf("%t %t", true, b)
	fmt.Printf("%T %T", 3, i)
	fmt.Printf("%U %U", 3, i)
	fmt.Printf("%v %v", 3, i)
	fmt.Printf("%x %x %x %x", 3, i, "hi", s)
	fmt.Printf("%X %X %X %X", 3, i, "hi", s)
	fmt.Printf("%.*s %d %g", 3, "hi", 23, 2.3)
	fmt.Printf("%s", &stringerv)
	fmt.Printf("%v", &stringerv)
	fmt.Printf("%T", &stringerv)
	fmt.Printf("%s", &embeddedStringerv)
	fmt.Printf("%v", &embeddedStringerv)
	fmt.Printf("%T", &embeddedStringerv)
	fmt.Printf("%v", notstringerv)
	fmt.Printf("%T", notstringerv)
	fmt.Printf("%q", stringerarrayv)
	fmt.Printf("%v", stringerarrayv)
	fmt.Printf("%s", stringerarrayv)
	fmt.Printf("%v", notstringerarrayv)
	fmt.Printf("%T", notstringerarrayv)
	fmt.Printf("%d", new(fmt.Formatter))
	fmt.Printf("%f", new(big.Float))
	fmt.Printf("%*%", 2)               // Ridiculous but allowed.
	fmt.Printf("%s", interface{}(nil)) // Nothing useful we can say.

	fmt.Printf("%g", 1+2i)
	fmt.Printf("%#e %#E %#f %#F %#g %#G", 1.2, 1.2, 1.2, 1.2, 1.2, 1.2) // OK since Go 1.9
	// Some bad format/argTypes
	fmt.Printf("%b", "hi")                      // want `Printf format %b has arg #1 of wrong type string`
	_ = fmt.Sprintf("%b", "hi")                 // want `Printf format %b has arg #1 of wrong type string`
	fmt.Fprintf(os.Stdout, "%b", "hi")          // want `Printf format %b has arg #1 of wrong type string`
	fmt.Printf("%t", c)                         // want `Printf format %t has arg #1 of wrong type complex64`
	fmt.Printf("%t", 1+2i)                      // want `Printf format %t has arg #1 of wrong type complex128`
	fmt.Printf("%c", 2.3)                       // want `Printf format %c has arg #1 of wrong type float64`
	fmt.Printf("%d", 2.3)                       // want `Printf format %d has arg #1 of wrong type float64`
	fmt.Printf("%e", "hi")                      // want `Printf format %e has arg #1 of wrong type string`
	fmt.Printf("%E", true)                      // want `Printf format %E has arg #1 of wrong type bool`
	fmt.Printf("%f", "hi")                      // want `Printf format %f has arg #1 of wrong type string`
	fmt.Printf("%F", 'x')                       // want `Printf format %F has arg #1 of wrong type rune`
	fmt.Printf("%g", "hi")                      // want `Printf format %g has arg #1 of wrong type string`
	fmt.Printf("%g", imap)                      // want `Printf format %g has arg #1 of wrong type map\[int\]int`
	fmt.Printf("%G", i)                         // want `Printf format %G has arg #1 of wrong type int`
	fmt.Printf("%o", x)                         // want `Printf format %o has arg #1 of wrong type float64`
	fmt.Printf("%p", 23)                        // want `Printf format %p has arg #1 of wrong type int`
	fmt.Printf("%q", x)                         // want `Printf format %q has arg #1 of wrong type float64`
	fmt.Printf("%s", b)                         // want `Printf format %s has arg #1 of wrong type bool`
	fmt.Printf("%s", byte(65))                  // want `Printf format %s has arg #1 of wrong type byte`
	fmt.Printf("%t", 23)                        // want `Printf format %t has arg #1 of wrong type int`
	fmt.Printf("%U", x)                         // want `Printf format %U has arg #1 of wrong type float64`
	fmt.Printf("%X", 2.3)                       // want `Printf format %X has arg #1 of wrong type float64`
	fmt.Printf("%s", stringerv)                 // want `Printf format %s has arg #1 of wrong type CheckPrintf\.ptrStringer`
	fmt.Printf("%t", stringerv)                 // want `Printf format %t has arg #1 of wrong type CheckPrintf\.ptrStringer`
	fmt.Printf("%s", embeddedStringerv)         // want `Printf format %s has arg #1 of wrong type CheckPrintf\.embeddedStringer`
	fmt.Printf("%t", embeddedStringerv)         // want `Printf format %t has arg #1 of wrong type CheckPrintf\.embeddedStringer`
	fmt.Printf("%q", notstringerv)              // want `Printf format %q has arg #1 of wrong type CheckPrintf\.notstringer`
	fmt.Printf("%t", notstringerv)              // want `Printf format %t has arg #1 of wrong type CheckPrintf\.notstringer`
	fmt.Printf("%t", stringerarrayv)            // want `Printf format %t has arg #1 of wrong type CheckPrintf\.stringerarray`
	fmt.Printf("%t", notstringerarrayv)         // want `Printf format %t has arg #1 of wrong type CheckPrintf\.notstringerarray`
	fmt.Printf("%q", notstringerarrayv)         // want `Printf format %q has arg #1 of wrong type CheckPrintf\.notstringerarray`
	fmt.Printf("%d", BoolFormatter(true))       // want `Printf format %d has arg #1 of wrong type CheckPrintf\.BoolFormatter`
	fmt.Printf("%z", FormatterVal(true))        // correct (the type is responsible for formatting)
	fmt.Printf("%d", FormatterVal(true))        // correct (the type is responsible for formatting)
	fmt.Printf("%s", nonemptyinterface)         // correct (the type is responsible for formatting)
	fmt.Printf("%.*s %d %6g", 3, "hi", 23, 'x') // want `Printf format %6g has arg #4 of wrong type rune`
	fmt.Printf("%s", "hi", 3)                   // want `Printf call needs 1 args but has 2 args`
	fmt.Printf("%"+("s"), "hi", 3)              // want `Printf call needs 1 args but has 2 args`
	fmt.Printf("%s%%%d", "hi", 3)               // correct
	fmt.Printf("%08s", "woo")                   // correct
	fmt.Printf("% 8s", "woo")                   // correct
	fmt.Printf("%.*d", 3, 3)                    // correct
	fmt.Printf("%.*d x", 3, 3, 3, 3)            // want `Printf call needs 2 args but has 4 args`
	fmt.Printf("%.*d x", "hi", 3)               // want `Printf format %\.\*d reads non-int arg #1 as argument of \*`
	fmt.Printf("%.*d x", i, 3)                  // correct
	fmt.Printf("%.*d x", s, 3)                  // want `Printf format %\.\*d reads non-int arg #1 as argument of \*`
	fmt.Printf("%*% x", 0.22)                   // want `Printf format %\*% reads non-int arg #1 as argument of \*`
	fmt.Printf("%q %q", multi()...)             // ok
	fmt.Printf("%#q", `blah`)                   // ok
	const format = "%s %s\n"
	fmt.Printf(format, "hi", "there")
	fmt.Printf(format, "hi")              // want `Printf format %s reads arg #2, but call has only 1 args`
	fmt.Printf("%s %d %.3v %q", "str", 4) // want `Printf format %\.3v reads arg #3, but call has only 2 args`

	fmt.Printf("%#s", FormatterVal(true)) // correct (the type is responsible for formatting)
	fmt.Printf("d%", 2)                   // want `couldn't parse format string`
	fmt.Printf("%d", percentDV)
	fmt.Printf("%d", &percentDV)
	fmt.Printf("%d", notPercentDV)  // want `Printf format %d has arg #1 of wrong type CheckPrintf\.notPercentDStruct`
	fmt.Printf("%d", &notPercentDV) // want `Printf format %d has arg #1 of wrong type \*CheckPrintf\.notPercentDStruct`
	fmt.Printf("%p", &notPercentDV) // Works regardless: we print it as a pointer.
	fmt.Printf("%q", &percentDV)    // want `Printf format %q has arg #1 of wrong type \*CheckPrintf\.percentDStruct`
	fmt.Printf("%s", percentSV)
	fmt.Printf("%s", &percentSV)
	// Good argument reorderings.
	fmt.Printf("%[1]d", 3)
	fmt.Printf("%[1]*d", 3, 1)
	fmt.Printf("%[2]*[1]d", 1, 3)
	fmt.Printf("%[2]*.[1]*[3]d", 2, 3, 4)
	fmt.Fprintf(os.Stderr, "%[2]*.[1]*[3]d", 2, 3, 4) // Use Fprintf to make sure we count arguments correctly.
	// Bad argument reorderings.
	fmt.Printf("%[xd", 3)                      // want `couldn't parse format string`
	fmt.Printf("%[x]d x", 3)                   // want `couldn't parse format string`
	fmt.Printf("%[3]*s x", "hi", 2)            // want `Printf format %\[3\]\*s reads arg #3, but call has only 2 args`
	fmt.Printf("%[3]d x", 2)                   // want `Printf format %\[3\]d reads arg #3, but call has only 1 args`
	fmt.Printf("%[2]*.[1]*[3]d x", 2, "hi", 4) // want `Printf format %\[2\]\*\.\[1\]\*\[3\]d reads non-int arg #2 as argument of \*`
	fmt.Printf("%[0]s x", "arg1")              // want `Printf format %\[0\]s reads invalid arg 0; indices are 1-based`
	fmt.Printf("%[0]d x", 1)                   // want `Printf format %\[0\]d reads invalid arg 0; indices are 1-based`

	// Interfaces can be used with any verb.
	var iface interface {
		ToTheMadness() bool // Method ToTheMadness usually returns false
	}
	fmt.Printf("%f", iface) // ok: fmt treats interfaces as transparent and iface may well have a float concrete type
	// Can print functions in many ways
	fmt.Printf("%s", someFunction) // want `Printf format %s has arg #1 of wrong type func\(\)`
	fmt.Printf("%d", someFunction) // ok: maybe someone wants to see the pointer
	fmt.Printf("%v", someFunction) // ok: maybe someone wants to see the pointer in decimal
	fmt.Printf("%p", someFunction) // ok: maybe someone wants to see the pointer
	fmt.Printf("%T", someFunction) // ok: maybe someone wants to see the type
	// Bug: used to recur forever.
	fmt.Printf("%p %x", recursiveStructV, recursiveStructV.next)
	fmt.Printf("%p %x", recursiveStruct1V, recursiveStruct1V.next)
	fmt.Printf("%p %x", recursiveSliceV, recursiveSliceV)
	//fmt.Printf("%p %x", recursiveMapV, recursiveMapV)

	// indexed arguments
	fmt.Printf("%d %[3]d %d %[2]d x", 1, 2, 3, 4)    // OK
	fmt.Printf("%d %[0]d %d %[2]d x", 1, 2, 3, 4)    // want `Printf format %\[0\]d reads invalid arg 0; indices are 1-based`
	fmt.Printf("%d %[3]d %d %[-2]d x", 1, 2, 3, 4)   // want `couldn't parse format string`
	fmt.Printf("%d %[3]d %d %[5]d x", 1, 2, 3, 4)    // want `Printf format %\[5\]d reads arg #5, but call has only 4 args`
	fmt.Printf("%d %[3]d %-10d %[2]d x", 1, 2, 3)    // want `Printf format %-10d reads arg #4, but call has only 3 args`
	fmt.Printf("%[1][3]d x", 1, 2)                   // want `couldn't parse format string`
	fmt.Printf("%[1]d x", 1, 2)                      // OK
	fmt.Printf("%d %[3]d %d %[2]d x", 1, 2, 3, 4, 5) // OK

	fmt.Printf(someString(), "hello") // OK

	// d accepts pointers as long as they're not to structs.
	// pointers to structs are dereferencd and walked.
	fmt.Printf("%d", &s)

	// staticcheck's own checks, based on bugs in go vet; see https://github.com/golang/go/issues/27672
	{

		type T2 struct {
			X string
		}

		type T1 struct {
			X *T2
		}
		x1 := []string{"hi"}
		t1 := T1{&T2{"hi"}}

		fmt.Printf("%s\n", &x1)
		fmt.Printf("%s\n", t1) // want `Printf format %s has arg #1 of wrong type CheckPrintf\.T1`
		var x2 struct{ A *int }
		fmt.Printf("%p\n", x2) // want `Printf format %p has arg #1 of wrong type struct\{A \*int\}`
		var x3 [2]int
		fmt.Printf("%p", x3) // want `Printf format %p has arg #1 of wrong type \[2\]int`

		ue := unexportedError{nil}
		fmt.Printf("%s", ue)
	}

	fmt.Printf("%s", Error(0))
}

func someString() string { return "X" }

// A function we use as a function value; it has no other purpose.
func someFunction() {}

// multi is used by the test.
func multi() []interface{} {
	panic("don't call - testing only")
}

type stringer int

func (stringer) String() string { return "string" }

type ptrStringer float64

var stringerv ptrStringer

func (*ptrStringer) String() string {
	return "string"
}

type embeddedStringer struct {
	foo string
	ptrStringer
	bar int
}

var embeddedStringerv embeddedStringer

type notstringer struct {
	f float64
}

var notstringerv notstringer

type stringerarray [4]float64

func (stringerarray) String() string {
	return "string"
}

var stringerarrayv stringerarray

type notstringerarray [4]float64

var notstringerarrayv notstringerarray

var nonemptyinterface = interface {
	f()
}(nil)

// A data type we can print with "%d".
type percentDStruct struct {
	a int
	b []byte
	c *float64
}

var percentDV percentDStruct

// A data type we cannot print correctly with "%d".
type notPercentDStruct struct {
	a int
	b []byte
	c bool
}

var notPercentDV notPercentDStruct

// A data type we can print with "%s".
type percentSStruct struct {
	a string
	b []byte
	C stringerarray
}

var percentSV percentSStruct

type BoolFormatter bool

func (*BoolFormatter) Format(fmt.State, rune) {
}

// Formatter with value receiver
type FormatterVal bool

func (FormatterVal) Format(fmt.State, rune) {
}

type RecursiveSlice []RecursiveSlice

var recursiveSliceV = &RecursiveSlice{}

type RecursiveMap map[int]RecursiveMap

var recursiveMapV = make(RecursiveMap)

type RecursiveStruct struct {
	next *RecursiveStruct
}

var recursiveStructV = &RecursiveStruct{}

type RecursiveStruct1 struct {
	next *RecursiveStruct2
}

type RecursiveStruct2 struct {
	next *RecursiveStruct1
}

var recursiveStruct1V = &RecursiveStruct1{}

type unexportedInterface struct {
	f interface{}
}

// Issue 17798: unexported ptrStringer cannot be formatted.
type unexportedStringer struct {
	t ptrStringer
}
type unexportedStringerOtherFields struct {
	s string
	t ptrStringer
	S string
}

// Issue 17798: unexported error cannot be formatted.
type unexportedError struct {
	e error
}
type unexportedErrorOtherFields struct {
	s string
	e error
	S string
}

type errorer struct{}

func (e errorer) Error() string { return "errorer" }

type unexportedCustomError struct {
	e errorer
}

type errorInterface interface {
	error
	ExtraMethod()
}

type unexportedErrorInterface struct {
	e errorInterface
}

func UnexportedStringerOrError() {
	fmt.Printf("%s", unexportedInterface{"foo"}) // ok; prints {foo}
	fmt.Printf("%s", unexportedInterface{3})     // ok; we can't see the problem

	us := unexportedStringer{}
	fmt.Printf("%s", us)  // want `Printf format %s has arg #1 of wrong type CheckPrintf\.unexportedStringer`
	fmt.Printf("%s", &us) // want `Printf format %s has arg #1 of wrong type \*CheckPrintf\.unexportedStringer`

	usf := unexportedStringerOtherFields{
		s: "foo",
		S: "bar",
	}
	fmt.Printf("%s", usf)  // want `Printf format %s has arg #1 of wrong type CheckPrintf\.unexportedStringerOtherFields`
	fmt.Printf("%s", &usf) // want `Printf format %s has arg #1 of wrong type \*CheckPrintf\.unexportedStringerOtherFields`

	intSlice := []int{3, 4}
	fmt.Printf("%s", intSlice) // want `Printf format %s has arg #1 of wrong type \[\]int`
	nonStringerArray := [1]unexportedStringer{{}}
	fmt.Printf("%s", nonStringerArray)  // want `Printf format %s has arg #1 of wrong type \[1\]CheckPrintf\.unexportedStringer`
	fmt.Printf("%s", []stringer{3, 4})  // not an error
	fmt.Printf("%s", [2]stringer{3, 4}) // not an error
}

// TODO: Disable complaint about '0' for Go 1.10. To be fixed properly in 1.11.
// See issues 23598 and 23605.
func DisableErrorForFlag0() {
	fmt.Printf("%0t", true)
}

// Issue 26486.
func dbg(format string, args ...interface{}) {
	if format == "" {
		format = "%v"
	}
	fmt.Printf(format, args...)
}
