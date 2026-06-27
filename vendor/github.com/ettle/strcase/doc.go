/*
Package strcase is a package for converting strings into various word cases
(e.g. snake_case, camelCase)

	go get -u github.com/ettle/strcase

Example usage

	strcase.ToSnake("Hello World")     // hello_world
	strcase.ToSNAKE("Hello World")     // HELLO_WORLD

	strcase.ToKebab("helloWorld")      // hello-world
	strcase.ToKEBAB("helloWorld")      // HELLO-WORLD

	strcase.ToPascal("hello-world")    // HelloWorld
	strcase.ToCamel("hello-world")     // helloWorld

	// Handle odd cases
	strcase.ToSnake("FOOBar")          // foo_bar

	// Support Go initialisms
	strcase.ToGoPascal("http_response") // HTTPResponse

	// Specify case and delimiter
	strcase.ToCase("HelloWorld", strcase.UpperCase, '.') // HELLO.WORLD

## Why this package

String strcase is pretty straight forward and there are a number of methods to
do it. This package is fully featured, more customizable, better tested, and
faster than other packages and what you would probably whip up yourself.

### Unicode support

We work for with unicode strings and pay very little performance penalty for it
as we optimized for the common use case of ASCII only strings.

### Customization

You can create a custom caser that changes the behavior to what you want. This
customization also reduces the pressure for us to change the default behavior
which means that things are more stable for everyone involved.  The goal is to
make the common path easy and fast, while making the uncommon path possible.

	 c := NewCaser(
		// Use Go's default initialisms e.g. ID, HTML
	 	true,
		// Override initialisms (e.g. don't initialize HTML but initialize SSL
	 	map[string]bool{"SSL": true, "HTML": false},
		// Write your own custom SplitFn
		//
	 	NewSplitFn(
	 		[]rune{'*', '.', ','},
	 		SplitCase,
	 		SplitAcronym,
	 		PreserveNumberFormatting,
	 		SplitBeforeNumber,
	 		SplitAfterNumber,
	 	))
	 assert.Equal(t, "http_200", c.ToSnake("http200"))

### Initialism support

By default, we use the golint intialisms list. You can customize and override
the initialisms if you wish to add additional ones, such as "SSL" or "CMS" or
domain specific ones to your industry.

	ToGoPascal("http_response") // HTTPResponse
	ToGoSnake("http_response") // HTTP_response

### Test coverage

We have a wide ranging test suite to make sure that we understand our behavior.
Test coverage isn't everything, but we aim for 100% coverage.

### Fast

Optimized to reduce memory allocations with Builder. Benchmarked and optimized
around common cases.

We're on par with the fastest packages (that have less features) and much
faster than others. We also benchmarked against code snippets. Using string
builders to reduce memory allocation and reordering boolean checks for the
common cases have a large performance impact.

Hopefully I was fair to each library and happy to rerun benchmarks differently
or reword my commentary based on suggestions or updates.

	// This package - faster then almost all libraries
	// Initialisms are more complicated and slightly slower, but still fast
	BenchmarkToTitle-96                      9617142               125.7 ns/op            16 B/op          1 allocs/op
	BenchmarkToSnake-96                     10659919               120.7 ns/op            16 B/op          1 allocs/op
	BenchmarkToSNAKE-96                      9018282               126.4 ns/op            16 B/op          1 allocs/op
	BenchmarkToGoSnake-96                    4903687               254.5 ns/op            26 B/op          4 allocs/op
	BenchmarkToCustomCaser-96                4434489               265.0 ns/op            28 B/op          4 allocs/op

	// Segment has very fast snake case and camel case libraries
	// No features or customization, but very very fast
	BenchmarkSegment-96                     33625734                35.54 ns/op           16 B/op          1 allocs/op

	// Iancoleman has gotten some performance improvements, but remains
	// without unicode support and lacks fine-grained customization
	BenchmarkToSnakeIan-96                  13141522                92.99 ns/op           16 B/op          1 allocs/op

	// Stdlib strings.Title is deprecated; using golang.org/x.text
	BenchmarkGolangOrgXTextCases-96          4665676               262.5 ns/op           272 B/op          2 allocs/op

	// Other libraries or code snippets
	// - Most are slower, by up to an order of magnitude
	// - No support for initialisms or customization
	// - Some generate only camelCase or snake_case
	// - Many lack unicode support
	BenchmarkToSnakeStoewer-96               8095468               148.9 ns/op            64 B/op          2 allocs/op
	// Copying small rune arrays is slow
	BenchmarkToSnakeSiongui-96               2912593               401.7 ns/op           112 B/op         19 allocs/op
	BenchmarkGoValidator-96                  3493800               342.6 ns/op           184 B/op          9 allocs/op
	// String alloction is slow
	BenchmarkToSnakeFatih-96                 1282648               945.1 ns/op           616 B/op         26 allocs/op
	// Regexp is slow
	BenchmarkToSnakeGolangPrograms-96         778674              1495 ns/op             227 B/op         11 allocs/op

	// These results aren't a surprise - my initial version of this library was
	// painfully slow. I think most of us, without spending some time with
	// profilers and benchmarks, would write also something on the slower side.

### Zero dependencies

That's right - zero. We only import the Go standard library. No hassles with
dependencies, licensing, security alerts.

## Why not this package

If every nanosecond matters and this is used in a tight loop, use segment.io's
libraries (https://github.com/segmentio/go-snakecase and
https://github.com/segmentio/go-camelcase). They lack features, but make up for
it by being blazing fast.

## Migrating from other packages

If you are migrating from from another package, you may find slight differences
in output. To reduce the delta, you may find it helpful to use the following
custom casers to mimic the behavior of the other package.

	// From https://github.com/iancoleman/strcase
	var c = NewCaser(false, nil, NewSplitFn([]rune{'_', '-', '.'}, SplitCase, SplitAcronym, SplitBeforeNumber))

	// From https://github.com/stoewer/go-strcase
	var c = NewCaser(false, nil, NewSplitFn([]rune{'_', '-'}, SplitCase), SplitAcronym)
*/
package strcase
