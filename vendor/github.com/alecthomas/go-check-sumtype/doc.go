/*
sumtype takes a list of Go package paths or files and looks for sum type
declarations in each package/file provided. Exhaustiveness checks are then
performed for each use of a declared sum type in a type switch statement.
Namely, sumtype will report an error for any type switch statement that
either lacks a default clause or does not account for all possible variants.

Declarations are provided in comments like so:

	//sumtype:decl
	type MySumType interface { ... }

MySumType must be *sealed*. That is, part of its interface definition contains
an unexported method.

sumtype will produce an error if any of the above is not true.

For valid declarations, sumtype will look for all occurrences in which a
value of type MySumType participates in a type switch statement. In those
occurrences, it will attempt to detect whether the type switch is exhaustive
or not. If it's not, sumtype will report an error. For example:

	$ cat mysumtype.go
	package gochecksumtype

	//sumtype:decl
	type MySumType interface {
		sealed()
	}

	type VariantA struct{}

	func (a *VariantA) sealed() {}

	type VariantB struct{}

	func (b *VariantB) sealed() {}

	func main() {
		switch MySumType(nil).(type) {
		case *VariantA:
		}
	}
	$ sumtype mysumtype.go
	mysumtype.go:18:2: exhaustiveness check failed for sum type 'MySumType': missing cases for VariantB

Adding either a default clause or a clause to handle *VariantB will cause
exhaustive checks to pass.

As a special case, if the type switch statement contains a default clause
that always panics, then exhaustiveness checks are still performed.
*/
package gochecksumtype
