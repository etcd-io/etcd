[![Go Report Card](https://goreportcard.com/badge/github.com/nunnatsa/ginkgolinter)](https://goreportcard.com/report/github.com/nunnatsa/ginkgolinter)
[![Coverage Status](https://coveralls.io/repos/github/nunnatsa/ginkgolinter/badge.svg?branch=main)](https://coveralls.io/github/nunnatsa/ginkgolinter?branch=main)
![Build Status](https://github.com/nunnatsa/ginkgolinter/workflows/CI/badge.svg)
[![License](https://img.shields.io/github/license/nunnatsa/ginkgolinter)](/LICENSE)
[![Release](https://img.shields.io/github/release/nunnatsa/ginkgolinter.svg)](https://github.com/nunnatsa/ginkgolinter/releases/latest)
[![GitHub Releases Stats of ginkgolinter](https://img.shields.io/github/downloads/nunnatsa/ginkgolinter/total.svg?logo=github)](https://somsubhra.github.io/github-release-stats/?username=nunnatsa&repository=ginkgolinter)

# ginkgo-linter
[ginkgo](https://onsi.github.io/ginkgo/) is a popular testing framework and [gomega](https://onsi.github.io/gomega/) is its assertion package.

This is a golang linter to enforce some standards while using the ginkgo and gomega packages.

## Install the CLI
Download the right executable from the latest release, according to your OS.

Another option is to use go:
```shell
go install github.com/nunnatsa/ginkgolinter/cmd/ginkgolinter@latest
```
Then add the new executable to your PATH.

## usage
```shell
ginkgolinter [-fix] ./...
```

Use the `-fix` flag to apply the fix suggestions to the source code.

### Use ginkgolinter with golangci-lint
The ginkgolinter is now part of the popular [golangci-lint](https://golangci-lint.run/), starting from version `v1.51.1`.

It is not enabled by default, though. There are two ways to run ginkgolinter with golangci-lint:

* From command line:
  ```shell
  golangci-lint run -E ginkgolinter ./...
  ```
* From configuration:
  
  Add ginkgolinter to the enabled linters list in .golangci.reference.yml file in your project. For more details, see
  the [golangci-lint documentation](https://golangci-lint.run/usage/configuration/); e.g.
   ```yaml
   linters:
     enable:
       - ginkgolinter
   ```
## Linter Rules
The linter checks the ginkgo and gomega assertions in golang test code. Gomega may be used together with ginkgo tests, 
For example:
```go
It("should test something", func() { // It is ginkgo test case function
	Expect("abcd").To(HaveLen(4), "the string should have a length of 4") // Expect is the gomega assertion
})
```
or within a classic golang test code, like this:
```go
func TestWithGomega(t *testing.T) {
	g := NewWithT(t)
	g.Expect("abcd").To(HaveLen(4), "the string should have a length of 4")
}
```

In some cases, the gomega will be passed as a variable to function by ginkgo, for example:
```go
Eventually(func(g Gomega) error {
	g.Expect("abcd").To(HaveLen(4), "the string should have a length of 4")
	return nil
}).Should(Succeed())
```

The linter checks the `Expect`, `ExpectWithOffset` and the `Ω` "actual" functions, with the `Should`, `ShouldNot`, `To`, `ToNot` and `NotTo` assertion functions.

It also supports the embedded `Not()` matcher

Some checks find actual bugs, and some are more for style.

### Using a function call in async assertion [BUG]
This rule finds an actual bug in tests, where asserting a function call in an async function; e.g. `Eventually`. For
example:
```go
func slowInt(int val) int {
	time.Sleep(time.Second)
	return val
}

...

It("should test that slowInt returns 42, eventually", func() {
	Eventually(slowInt(42)).WithPolling(time.Millisecond * 100).WithTimeout(time.Second * 2).Equal(42)
})
```
The problem with the above code is that it **should** poll - call the function - until it returns 42, but what actually
happens is that first the function is called, and we pass `42` to `Eventually` - not the function. This is not what we
tried to do here.

The linter will suggest replacing this code by:
```go
It("should test that slowInt returns 42, eventually", func() {
	Eventually(slowInt).WithArguments(42).WithPolling(time.Millisecond * 100).WithTimeout(time.Second * 2).Equal(42)
})
```

The linter suggested replacing the function call by the function name.

If function arguments are used, the linter will add the `WithArguments()` method to pass them.

Please notice that `WithArguments()` is only supported from gomenga v1.22.0.

When using an older version of gomega, change the code manually. For example:

```go
It("should test that slowInt returns 42, eventually", func() {
	Eventually(func() int {
		slowint(42)		
	}).WithPolling(time.Millisecond * 100).WithTimeout(time.Second * 2).Equal(42)
})
```

### Comparing a pointer with a value [BUG]
The linter warns when comparing a pointer with a value.
These comparisons are always wrong and will always fail.

In case of a positive assertion (`To()` or `Should()`), the test will just fail.

But the main concern is for false positive tests, when using a negative assertion (`NotTo()`, `ToNot()`, `ShouldNot()`,
`Should(Not())` etc.); e.g.
```go
num := 5
...
pNum := &num
...
Expect(pNum).ShouldNot(Equal(6))
```
This assertion will pass, but for the wrong reasons: pNum is not equal 6, not because num == 5, but because pNum is
a pointer, while `6` is an `int`.

In the case above, the linter will suggest `Expect(pNum).ShouldNot(HaveValue(Equal(6)))`

This is also right for additional matchers: `BeTrue()` and `BeFalse()`, `BeIdenticalTo()`, `BeEquivalentTo()`
and `BeNumerically`.

### Missing Assertion Method [BUG]
The linter warns when calling an "actual" method (e.g. `Expect()`, `Eventually()` etc.), without an assertion method (e.g
`Should()`, `NotTo()` etc.)

For example:
```go
// no assertion for the result
Eventually(doSomething).WithTimeout(time.Seconds * 5).WithPolling(time.Milliseconds * 100)
```

The linter will not suggest a fix for this warning.

This rule cannot be suppressed.

### Focus Container / Focus individual spec found [BUG]
This rule finds ginkgo focus containers, or the `Focus` individual spec in the code.

ginkgo supports the `FDescribe`, `FContext`, `FWhen`, `FIt`, `FDescribeTable` and `FEntry`
containers to allow the developer to focus
on a specific test or set of tests during test development or debug.

For example:
```go
var _ = Describe("checking something", func() {
    FIt("this test is the only one that will run", func(){
        ...
    })
})
```
Alternatively, the `Focus` individual spec may be used for the same purpose, e.g.
```go
var _ = Describe("checking something", Focus, func() {
    It("this test is the only one that will run", func(){
        ...
    })
})
```

These container, or the `Focus` spec, must not be part of the final source code, and should only be used locally by the 
developer.

***This rule is disabled by default***. Use the `--forbid-focus-container` command line flag to enable it.  

### Comparing values from different types [BUG]

The `Equal` and the `BeIdentical` matchers also check the type, not only the value.
    
The following code will fail in runtime:    
```go
x := 5 // x is int
Expect(x).Should(Equal(uint(5)) // x and uint(5) are with different
```
When using negative checks, it's even worse, because we get a false positive:
```
x := 5
Expect(x).ShouldNot(Equal(uint(5))
```

The linter suggests two options to solve this warning: either compare with the same type, e.g. 
using casting, or use the `BeEquivalentTo` matcher.

The linter can't guess what is the best solution in each case, and so it won't auto-fix this warning.

To suppress this warning entirely, use the `--suppress-type-compare-assertion` command line parameter. 

To suppress a specific file or line, use the `// ginkgo-linter:ignore-type-compare-warning` comment (see [below](#suppress-warning-from-the-code))

### Wrong Usage of the `MatchError` gomega Matcher [BUG]
The `MatchError` gomega matcher asserts an error value (and if it's not nil).
There are four valid formats for using this Matcher:
* error value; e.g. `Expect(err).To(MatchError(anotherErr))`
* string, to be equal to the output of the `Error()` method; e.g. `Expect(err).To(MatchError("Not Found"))`
* A gomega matcher that asserts strings; e.g. `Expect(err).To(MatchError(ContainSubstring("Found")))`
* [from v0.29.0] a function that receive a single error parameter and returns a single boolean value. 
  In this format, an additional single string parameter, with the function description, is also required; e.g.
  `Expect(err).To(MatchError(isNotFound, "is the error is a not found error"))`

These four format are checked on runtime, but sometimes it's too late. ginkgolinter performs a static analysis and so it
will find these issues on build time.

ginkgolinter checks the following:
* Is the first parameter is one of the four options above.
* That there are no additional parameters passed to the matcher; e.g.
  `MatchError(isNotFoundFunc, "a valid description" , "not used string")`. In this case, the matcher won't fail on run 
  time, but the additional parameters are not in use and ignored.   
* If the first parameter is a function with the format of `func(error)bool`, ginkgolinter makes sure that the second 
  parameter exists and its type is string.

### Async timing interval: timeout is shorter than polling interval [BUG]
***Note***: Only applied when the `suppress-async-assertion` flag is **not set** *and* the `validate-async-intervals` 
flag **is** set.

***Note***: This rule work with best-effort approach. It can't find many cases, like const defined not in the same
package, or when using variables.

The timeout and polling intervals may be passed as optional arguments to the `Eventually` or `Consistently` functions, or
using the `WithTimeout` or , `Within` methods (timeout), and `WithPolling` or `ProbeEvery` methods (polling).

This rule checks if the async (`Eventually` or `Consistently`) timeout duration, is not shorter than the polling interval.

For example:
   ```go
   Eventually(aFunc).WithTimeout(500 * time.Millisecond).WithPolling(10 * time.Second).Should(Succeed())
   ```

This will probably happen when using the old format:
   ```go
   Eventually(aFunc, 500 * time.Millisecond /*timeout*/, 10 * time.Second /*polling*/).Should(Succeed())
   ```

### Prevent Wrong Actual Values with the Succeed() matcher [Bug]
The `Succeed()` matcher only accepts a single error value. this rule validates that. 

For example:
   ```go
   Expect(42).To(Succeed())
   ```

But mostly, we want to avoid using this matcher with functions that return multiple values, even if their last 
returned value is an error, because this is not supported:
   ```go
   Expect(os.Open("myFile.txt")).To(Succeed())
   ```

In async assertions (like `Eventually()`), the `Succeed()` matcher may also been used with functions that accept 
a Gomega object as their first parameter, and returns nothing, e.g. this is a valid usage of `Eventually`
  ```go
  Eventually(func(g Gomega){
	  g.Expect(true).To(BeTrue())
  }).WithTimeout(10 * time.Millisecond).WithPolling(time.Millisecond).Should(Succeed())
  ```

***Note***: This rule **does not** support auto-fix.

### Avoid Spec Pollution: Don't Initialize Variables in Container Nodes [BUG/STYLE]:
***Note***: Only applied when the `--forbid-spec-pollution` flag is set (disabled by default).

According to [ginkgo documentation](https://onsi.github.io/ginkgo/#avoid-spec-pollution-dont-initialize-variables-in-container-nodes), 
no variable should be assigned within a container node (`Describe`, `Context`, `When` or their `F`, `P` or `X` forms)
  
For example:
```go
var _ = Describe("description", func(){
    var x = 10
    ...
})
```

Instead, use `BeforeEach()`; e.g.
```go
var _ = Describe("description", func (){
    var x int
	
    BeforeEach(func (){
        x = 10
    })
    ...
})
```

### Wrong Length Assertion [STYLE]
The linter finds assertion of the golang built-in `len` function, with all kind of matchers, while there are already 
gomega matchers for these usecases; We want to assert the item, rather than its length.

There are several wrong patterns:
```go
Expect(len(x)).To(Equal(0)) // should be: Expect(x).To(BeEmpty())
Expect(len(x)).To(BeZero()) // should be: Expect(x).To(BeEmpty())
Expect(len(x)).To(BeNumeric(">", 0)) // should be: Expect(x).ToNot(BeEmpty())
Expect(len(x)).To(BeNumeric(">=", 1)) // should be: Expect(x).ToNot(BeEmpty())
Expect(len(x)).To(BeNumeric("==", 0)) // should be: Expect(x).To(BeEmpty())
Expect(len(x)).To(BeNumeric("!=", 0)) // should be: Expect(x).ToNot(BeEmpty())

Expect(len(x)).To(Equal(1)) // should be: Expect(x).To(HaveLen(1))
Expect(len(x)).To(BeNumeric("==", 2)) // should be: Expect(x).To(HaveLen(2))
Expect(len(x)).To(BeNumeric("!=", 3)) // should be: Expect(x).ToNot(HaveLen(3))
```

It also supports the embedded `Not()` matcher; e.g.

`Ω(len(x)).Should(Not(Equal(4)))` => `Ω(x).ShouldNot(HaveLen(4))`

Or even (double negative):

`Ω(len(x)).To(Not(BeNumeric(">", 0)))` => `Ω(x).To(BeEmpty())`

The output of the linter,when finding issues, looks like this:
```
./testdata/src/a/a.go:14:5: ginkgo-linter: wrong length assertion; consider using `Expect("abcd").Should(HaveLen(4))` instead
./testdata/src/a/a.go:18:5: ginkgo-linter: wrong length assertion; consider using `Expect("").Should(BeEmpty())` instead
./testdata/src/a/a.go:22:5: ginkgo-linter: wrong length assertion; consider using `Expect("").Should(BeEmpty())` instead
```

### Wrong Cap Assertion [STYLE]
The linter finds assertion of the golang built-in `cap` function, with all kind of matchers, while there are already
gomega matchers for these usecases; We want to assert the item, rather than its cap.

There are several wrong patterns:
```go
Expect(cap(x)).To(Equal(0)) // should be: Expect(x).To(HaveCap(0))
Expect(cap(x)).To(BeZero()) // should be: Expect(x).To(HaveCap(0))
Expect(cap(x)).To(BeNumeric(">", 0)) // should be: Expect(x).ToNot(HaveCap(0))
Expect(cap(x)).To(BeNumeric("==", 2)) // should be: Expect(x).To(HaveCap(2))
Expect(cap(x)).To(BeNumeric("!=", 3)) // should be: Expect(x).ToNot(HaveCap(3))
```

#### use the `HaveLen(0)` matcher.  [STYLE]
The linter will also warn about the `HaveLen(0)` matcher, and will suggest to replace it with `BeEmpty()`

### Wrong `nil` Assertion [STYLE]
The linter finds assertion of the comparison to nil, with all kind of matchers, instead of using the existing `BeNil()` 
matcher; We want to assert the item, rather than a comparison result.

There are several wrong patterns:

```go
Expect(x == nil).To(Equal(true)) // should be: Expect(x).To(BeNil())
Expect(nil == x).To(Equal(true)) // should be: Expect(x).To(BeNil())
Expect(x != nil).To(Equal(true)) // should be: Expect(x).ToNot(BeNil())
Expect(nil != nil).To(Equal(true)) // should be: Expect(x).ToNot(BeNil())

Expect(x == nil).To(BeTrue()) // should be: Expect(x).To(BeNil())
Expect(x == nil).To(BeFalse()) // should be: Expect(x).ToNot(BeNil())
```
It also supports the embedded `Not()` matcher; e.g.

`Ω(x == nil).Should(Not(BeTrue()))` => `Ω(x).ShouldNot(BeNil())`

Or even (double negative):

`Ω(x != nil).Should(Not(BeTrue()))` => `Ω(x).Should(BeNil())`

### Wrong boolean Assertion [STYLE]
The linter finds assertion using the `Equal` method, with the values of to `true` or `false`, instead
of using the existing `BeTrue()` or `BeFalse()` matcher.

There are several wrong patterns:

```go
Expect(x).To(Equal(true)) // should be: Expect(x).To(BeTrue())
Expect(x).To(Equal(false)) // should be: Expect(x).To(BeFalse())
```
It also supports the embedded `Not()` matcher; e.g.

`Ω(x).Should(Not(Equal(True)))` => `Ω(x).ShouldNot(BeTrue())`

### Wrong Error Assertion [STYLE]
The linter finds assertion of errors compared with nil, or to be equal nil, or to be nil. The linter suggests to use `Succeed` for functions or `HaveOccurred` for error values..

There are several wrong patterns:

```go
Expect(err).To(BeNil()) // should be: Expect(err).ToNot(HaveOccurred())
Expect(err == nil).To(Equal(true)) // should be: Expect(err).ToNot(HaveOccurred())
Expect(err == nil).To(BeFalse()) // should be: Expect(err).To(HaveOccurred())
Expect(err != nil).To(BeTrue()) // should be: Expect(err).To(HaveOccurred())
Expect(funcReturnsError()).To(BeNil()) // should be: Expect(funcReturnsError()).To(Succeed())

and so on
```
It also supports the embedded `Not()` matcher; e.g.

`Ω(err == nil).Should(Not(BeTrue()))` => `Ω(x).Should(HaveOccurred())`

### Wrong Comparison Assertion [STYLE]
The linter finds assertion of boolean comparisons, which are already supported by existing gomega matchers. 

The linter assumes that when compared something to literals or constants, these values should be used for the assertion,
and it will do its best to suggest the right assertion expression accordingly. 

There are several wrong patterns:
```go
var x = 10
var s = "abcd"

...

Expect(x == 10).Should(BeTrue()) // should be Expect(x).Should(Equal(10))
Expect(10 == x).Should(BeTrue()) // should be Expect(x).Should(Equal(10))
Expect(x != 5).Should(Equal(true)) // should be Expect(x).ShouldNot(Equal(5))
Expect(x != 0).Should(Equal(true)) // should be Expect(x).ShouldNot(BeZero())

Expect(s != "abcd").Should(BeFalse()) // should be Expect(s).Should(Equal("abcd"))
Expect("abcd" != s).Should(BeFalse()) // should be Expect(s).Should(Equal("abcd"))
```
Or non-equal comparisons:
```go
Expect(x > 10).To(BeTrue()) // ==> Expect(x).To(BeNumerically(">", 10))
Expect(x >= 15).To(BeTrue()) // ==> Expect(x).To(BeNumerically(">=", 15))
Expect(3 > y).To(BeTrue()) // ==> Expect(y).To(BeNumerically("<", 3))
// and so on ...
```

This check included a limited support in constant values. For example:
```go
const c1 = 5

...

Expect(x1 == c1).Should(BeTrue()) // ==> Expect(x1).Should(Equal(c1))
Expect(c1 == x1).Should(BeTrue()) // ==> Expect(x1).Should(Equal(c1))
```

### Don't Allow Using `Expect` with `Should` or `ShouldNot` [STYLE]
This optional rule forces the usage of the `Expect` method only with the `To`, `ToNot` or `NotTo` 
assertion methods; e.g.
```go
Expect("abc").Should(HaveLen(3)) // => Expect("abc").To(HaveLen(3))
Expect("abc").ShouldNot(BeEmpty()) // => Expect("abc").ToNot(BeEmpty())
```
This rule support auto fixing.

***This rule is disabled by default***. Use the `--force-expect-to` command line flag to enable it.

### Async timing interval: multiple timeout or polling intervals [STYLE]
***Note***: Only applied when the `suppress-async-assertion` flag is **not set** *and* the `validate-async-intervals`
flag **is** set.

The timeout and polling intervals may be passed as optional arguments to the `Eventually` or `Consistently` functions, or
using the `WithTimeout` or , `Within` methods (timeout), and `WithPolling` or `ProbeEvery` methods (polling).

The linter checks that there is up to one polling argument and up to one timeout argument. 

For example:

```go
// both WithTimeout() and Within()
Eventually(aFunc).WithTimeout(time.Second * 10).Within(time.Second * 10).WithPolling(time.Millisecond * 500).Should(BeTrue())
// both polling argument, and WithPolling() method
Eventually(aFunc, time.Second*10, time.Millisecond * 500).WithPolling(time.Millisecond * 500).Should(BeTrue())
```

### Async timing interval: non-time.Duration intervals [STYLE]
***Note***: Only applied when the `suppress-async-assertion` flag is **not set** *and* the `validate-async-intervals`
flag **is** set.

gomega supports a few formats for timeout and polling intervals, when using the old format (the last two parameters of Eventually and Consistently):
* a `time.Duration` value
* any kind of numeric value (int(8/16/32/64), uint(8/16/32/64) or float(32/64), as the number of seconds.
* duration string like `"12s"`

The linter triggers a warning for any duration value that is not of the `time.Duration` type, assuming that this is
the desired type, given the type of the argument of the newer "WithTimeout", "WithPolling", "Within" and "ProbeEvery"
methods.

For example:
   ```go
   Eventually(func() bool { return true }, "1s").Should(BeTrue())
   Eventually(context.Background(), func() bool { return true }, time.Second*60, float64(2)).Should(BeTrue())
   ```

This rule offers a limited auto fix: for integer values, or integer consts, the linter will suggest multiply the
value with `time.Second`; e.g.
```go
const polling = 1
Eventually(aFunc, 5, polling)
```
will be changed to:
```go
Eventually(aFunc, time.Second*5, time.Second*polling)
```

### Correct usage of the `Succeed()` and the `HaveOccurred()` matchers
This rule enforces using the `Success()` matcher only for functions, and the `HaveOccurred()` matcher only for error
values.

For example:
  ```go
  Expect(err).To(Succeed())
  ```
will trigger a warning with a suggestion to replace the mather to
  ```go
  Expect(err).ToNot(HaveOccurred())
  ```

and vice versa:
  ```go
  Expect(myErrorFunc()).ToNot(HaveOccurred())
  ```
will trigger a warning with a suggestion to replace the mather to
  ```go
  Expect(myErrorFunc()).To(Succeed())
  ```
***This rule is disabled by default***. Use the `--force-succeed` command line flag to enable it.

***Note***: This rule **does** support auto-fix, when the `--fix` command line parameter is used.

### Force assertion descriptions [STYLE]
This rule enforces that all assertions include an optional description message to improve test readability and debugging.

```go
It("should test something", func() {
    Expect("hello").To(Equal("hello")) // This will trigger a warning
    Eventually(func() bool { return true }).Should(BeTrue()) // This will trigger a warning
})
```

Should be:
```go
It("should test something", func() {
    Expect("hello").To(Equal("hello"), "greeting should match")
    Eventually(func() bool { return true }).Should(BeTrue(), "condition should eventually be true")
})
```

The rule also works with async assertions and `Expect` calls inside `Eventually`:
```go
Eventually(func() {
    Expect(value).To(Equal(expected)) // This will also trigger a warning if no description
}).Should(Succeed(), "operation should complete successfully")
```

***This rule is disabled by default***. Use the `--force-assertion-description` command line flag to enable it.

### Force NotTo() [STYLE]
This rule enforces using the `NotTo()` or `ShouldNot()` assertion methods instead of `To(Not())` 
or `Should(Not())`.

***This rule is disabled by default***. Use the `--force-tonot` command line flag to enable it.

## Suppress the linter
### Suppress warning from command line
* Use the `--suppress-len-assertion` flag to suppress the wrong length and cap assertions warning
* Use the `--suppress-nil-assertion` flag to suppress the wrong nil assertion warning
* Use the `--suppress-err-assertion` flag to suppress the wrong error assertion warning
* Use the `--suppress-compare-assertion` flag to suppress the wrong comparison assertion warning
* Use the `--suppress-async-assertion` flag to suppress the function call in async assertion warning
* Use the `--forbid-focus-container` flag to activate the focused container assertion (deactivated by default)
* Use the `--suppress-type-compare-assertion` to suppress the type compare assertion warning
* Use the `--allow-havelen-0` flag to avoid warnings about `HaveLen(0)`; Note: this parameter is only supported from
  command line, and not from a comment.

### Suppress warning from the code
To suppress the wrong length and cap assertions warning, add a comment with (only)

`ginkgo-linter:ignore-len-assert-warning`. 

To suppress the wrong nil assertion warning, add a comment with (only)

`ginkgo-linter:ignore-nil-assert-warning`. 

To suppress the wrong error assertion warning, add a comment with (only)

`ginkgo-linter:ignore-err-assert-warning`. 

To suppress the wrong comparison assertion warning, add a comment with (only)

`ginkgo-linter:ignore-compare-assert-warning`. 

To suppress the wrong async assertion warning, add a comment with (only)

`ginkgo-linter:ignore-async-assert-warning`. 

To suppress the focus container warning, add a comment with (only)

`ginkgo-linter:ignore-focus-container-warning`

To suppress the different type comparison, add a comment with (only)

`ginkgo-linter:ignore-type-compare-warning`

Notice that this comment will not work for an anonymous variable container like
```go
// ginkgo-linter:ignore-focus-container-warning (not working!!)
var _ = FDescribe(...)
```
In this case, use the file comment (see below).

There are two options to use these comments:
1. If the comment is at the top of the file, suppress the warning for the whole file; e.g.:
   ```go
   package mypackage
   
   // ginkgo-linter:ignore-len-assert-warning
   
   import (
       . "github.com/onsi/ginkgo/v2"
       . "github.com/onsi/gomega"
   )
   
   var _ = Describe("my test", func() {
        It("should do something", func() {
            Expect(len("abc")).Should(Equal(3)) // nothing in this file will trigger the warning
        })
   })
   ```
   
2. If the comment is before a wrong length check expression, the warning is suppressed for this expression only; for example:
   ```golang
   It("should test something", func() {
       // ginkgo-linter:ignore-nil-assert-warning
       Expect(x == nil).Should(BeTrue()) // this line will not trigger the warning
       Expect(x == nil).Should(BeTrue()) // this line will trigger the warning
   }
   ```
