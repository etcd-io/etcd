/*
Package exhaustive defines an analyzer that checks exhaustiveness of switch
statements of enum-like constants in Go source code. The analyzer can
optionally also check exhaustiveness of keys in map literals whose key type
is enum-like.

# Definition of enum

The Go [language spec] does not have an explicit definition for enums. For
the purpose of this analyzer, and by convention, an enum type is any named
type that:

  - has an [underlying type] of float, string, or integer (includes byte
    and rune); and
  - has at least one constant of its type defined in the same [block].

In the example below, Biome is an enum type. The three constants are its
enum members.

	package eco

	type Biome int

	const (
		Tundra  Biome = 1
		Savanna Biome = 2
		Desert  Biome = 3
	)

Enum member constants for an enum type must be declared in the same block as
the type. The constant values may be specified using iota, literal values, or
any valid means for declaring a Go constant. It is allowed for multiple enum
member constants for an enum type to have the same constant value.

# Definition of exhaustiveness

A switch statement that switches on a value of an enum type is exhaustive if
all enum members are listed in the switch statement's cases. If multiple enum
members have the same constant value, it is sufficient for any one of these
same-valued members to be listed.

For an enum type defined in the same package as the switch statement, both
exported and unexported enum members must be listed to satisfy exhaustiveness.
For an enum type defined in an external package, it is sufficient that only
exported enum members are listed. Only constant identifiers (e.g. Tundra,
eco.Desert) listed in a switch statement's case clause can contribute towards
satisfying exhaustiveness; other expressions, such as literal values and
function calls, listed in case clauses do not contribute towards satisfying
exhaustiveness.

By default, the existence of a default case in a switch statement does not
unconditionally make a switch statement exhaustive. Use the
-default-signifies-exhaustive flag to adjust this behavior.

For a map literal whose key type is an enum type, a similar definition of
exhaustiveness applies. The map literal is considered exhaustive if all enum
members are be listed in its keys. Empty map literals are never checked for
exhaustiveness.

# Type parameters

A switch statement that switches on a value whose type is a type parameter is
checked for exhaustiveness if and only if each type element in the type
constraint is an enum type and the type elements share the same underlying
[BasicKind].

For example, the switch statement below will be checked because each type
element (i.e. M and N) in the type constraint is an enum type and the type
elements share the same underlying BasicKind, namely int8. To satisfy
exhaustiveness, the enum members collectively belonging to the enum types M
and N (i.e. A, B, and C) must be listed in the switch statement's cases.

	func bar[T M | I](v T) {
		switch v {
		case T(A):
		case T(B):
		case T(C):
		}
	}

	type I interface{ N }

	type M int8
	const A M = 1

	type N int8
	const B N = 2
	const C N = 3

# Type aliases

The analyzer handles type aliases as shown in the example below. newpkg.M is
an enum type. oldpkg.M is an alias for newpkg.M. Note that oldpkg.M isn't
itself an enum type; oldpkg.M is simply an alias for the actual enum type
newpkg.M.

	package oldpkg
	type M = newpkg.M
	const (
		A = newpkg.A
		B = newpkg.B
	)

	package newpkg
	type M int
	const (
		A M = 1
		B M = 2
	)

A switch statement that switches either on a value of type newpkg.M or of type
oldpkg.M (which, being an alias, is just an alternative spelling for newpkg.M)
is exhaustive if all of newpkg.M's enum members are listed in the switch
statement's cases. The following switch statement is exhaustive.

	func f(v newpkg.M) {
		switch v {
		case newpkg.A: // or equivalently oldpkg.A
		case newpkg.B: // or equivalently oldpkg.B
		}
	}

The analyzer guarantees that introducing a type alias (such as type M =
newpkg.M) will not result in new diagnostics if the set of enum member
constant values of the RHS type is a subset of the set of enum member constant
values of the LHS type.

# Flags

Summary:

	flag                           type                     default value
	----                           ----                     -------------
	-check                         comma-separated strings  switch
	-explicit-exhaustive-switch    bool                     false
	-explicit-exhaustive-map       bool                     false
	-check-generated               bool                     false
	-default-signifies-exhaustive  bool                     false
	-ignore-enum-members           regexp pattern           (none)
	-ignore-enum-types             regexp pattern           (none)
	-package-scope-only            bool                     false

Descriptions:

	-check
		Comma-separated list of program elements to check for
		exhaustiveness.  Supported program element values are
		"switch" and "map". The default value is "switch", which
		means that only switch statements are checked.

	-explicit-exhaustive-switch
		Check a switch statement only if it is associated with a
		"//exhaustive:enforce" comment. By default the analyzer
		checks every switch statement that isn't associated with a
		"//exhaustive:ignore" comment.

	-explicit-exhaustive-map
		Similar to -explicit-exhaustive-switch but for map literals.

	-check-generated
		Check generated files. For the definition of a generated
		file, see https://golang.org/s/generatedcode.

	-default-signifies-exhaustive
		Consider a switch statement to be exhaustive
		unconditionally if it has a default case. (In other words,
		all enum members do not have to be listed in its cases if a
		default case is present.) Setting this flag usually is
		counter to the purpose of exhaustiveness checks, so it is
		not recommended to set this flag.

	-ignore-enum-members
		Constants that match the specified regular expression (in
		package regexp syntax) are not considered enum members and
		hence do not have to be listed to satisfy exhaustiveness.
		The specified regular expression is matched against the
		constant name inclusive of import path. For example, if the
		import path for the constant is "example.org/eco" and the
		constant name is "Tundra", then the specified regular
		expression is matched against the string
		"example.org/eco.Tundra".

	-ignore-enum-types
		Similar to -ignore-enum-members but for types.

	-package-scope-only
		Only discover enums declared in file-level blocks. By
		default, the analyzer discovers enums defined in all
		blocks.

# Skip analysis

To skip analysis of a switch statement or a map literal, associate it with a
comment that begins with "//exhaustive:ignore". For example:

	//exhaustive:ignore ... an optional explanation goes here ...
	switch v {
	case A:
	case B:
	}

To ignore specific constants in exhaustiveness checks, specify the
-ignore-enum-members flag:

	exhaustive -ignore-enum-members '^example\.org/eco\.Tundra$'

To ignore specific types, specify the -ignore-enum-types flag:

	exhaustive -ignore-enum-types '^time\.Duration$|^example\.org/measure\.Unit$'

[language spec]: https://golang.org/ref/spec
[underlying type]: https://golang.org/ref/spec#Underlying_types
[block]: https://golang.org/ref/spec#Blocks
[BasicKind]: https://pkg.go.dev/go/types#BasicKind
*/
package exhaustive
