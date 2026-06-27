# regexp2 - full featured regular expressions for Go
Regexp2 is a feature-rich RegExp engine for Go.  It doesn't have constant time guarantees like the built-in `regexp` package, but it allows backtracking and is compatible with Perl5 and .NET.  You'll likely be better off with the RE2 engine from the `regexp` package and should only use this if you need to write very complex patterns or require compatibility with .NET.

## Basis of the engine
The engine is ported from the .NET framework's System.Text.RegularExpressions.Regex engine.  That engine was open sourced in 2015 under the MIT license.  There are some fundamental differences between .NET strings and Go strings that required a bit of borrowing from the Go framework regex engine as well.  I cleaned up a couple of the dirtier bits during the port (regexcharclass.cs was terrible), but the parse tree, code emmitted, and therefore patterns matched should be identical.

## New Code Generation
For extra performance use `regexp2` with [`regexp2cg`](https://github.com/dlclark/regexp2cg). It is a code generation utility for `regexp2` and you can likely improve your regexp runtime performance by 3-10x in hot code paths. As always you should benchmark your specifics to confirm the results. Give it a try!

## Installing
This is a go-gettable library, so install is easy:

    go get github.com/dlclark/regexp2

To use the new Code Generation (while it's in beta) you'll need to use the `code_gen` branch:

    go get github.com/dlclark/regexp2@code_gen

## Usage
Usage is similar to the Go `regexp` package.  Just like in `regexp`, you start by converting a regex into a state machine via the `Compile` or `MustCompile` methods.  They ultimately do the same thing, but `MustCompile` will panic if the regex is invalid.  You can then use the provided `Regexp` struct to find matches repeatedly.  A `Regexp` struct is safe to use across goroutines.

```go
re := regexp2.MustCompile(`Your pattern`, 0)
if isMatch, _ := re.MatchString(`Something to match`); isMatch {
    //do something
}
```

The only error that the `*Match*` methods *should* return is a Timeout if you set the `re.MatchTimeout` field.  Any other error is a bug in the `regexp2` package.  If you need more details about capture groups in a match then use the `FindStringMatch` method, like so:

```go
if m, _ := re.FindStringMatch(`Something to match`); m != nil {
    // the whole match is always group 0
    fmt.Printf("Group 0: %v\n", m.String())

    // you can get all the groups too
    gps := m.Groups()

    // a group can be captured multiple times, so each cap is separately addressable
    fmt.Printf("Group 1, first capture", gps[1].Captures[0].String())
    fmt.Printf("Group 1, second capture", gps[1].Captures[1].String())
}
```

Group 0 is embedded in the Match.  Group 0 is an automatically-assigned group that encompasses the whole pattern.  This means that `m.String()` is the same as `m.Group.String()` and `m.Groups()[0].String()`

The __last__ capture is embedded in each group, so `g.String()` will return the same thing as `g.Capture.String()` and  `g.Captures[len(g.Captures)-1].String()`.

If you want to find multiple matches from a single input string you should use the `FindNextMatch` method.  For example, to implement a function similar to `regexp.FindAllString`:

```go
func regexp2FindAllString(re *regexp2.Regexp, s string) []string {
	var matches []string
	m, _ := re.FindStringMatch(s)
	for m != nil {
		matches = append(matches, m.String())
		m, _ = re.FindNextMatch(m)
	}
	return matches
}
```

`FindNextMatch` is optmized so that it re-uses the underlying string/rune slice.

The internals of `regexp2` always operate on `[]rune` so `Index` and `Length` data in a `Match` always reference a position in `rune`s rather than `byte`s (even if the input was given as a string). This is a dramatic difference between `regexp` and `regexp2`.  It's advisable to use the provided `String()` methods to avoid having to work with indices.

## Compare `regexp` and `regexp2`
| Category | regexp | regexp2 |
| --- | --- | --- |
| Catastrophic backtracking possible | no, constant execution time guarantees | yes, if your pattern is at risk you can use the `re.MatchTimeout` field |
| Python-style capture groups `(?P<name>re)` | yes | no (yes in RE2 compat mode) |
| .NET-style capture groups `(?<name>re)` or `(?'name're)` | yes | yes |
| comments `(?#comment)` | no | yes |
| branch numbering reset `(?\|a\|b)` | no | no |
| possessive match `(?>re)` | no | yes |
| positive lookahead `(?=re)` | no | yes |
| negative lookahead `(?!re)` | no | yes |
| positive lookbehind `(?<=re)` | no | yes |
| negative lookbehind `(?<!re)` | no | yes |
| back reference `\1` | no | yes |
| named back reference `\k'name'` | no | yes |
| named ascii character class `[[:foo:]]`| yes | no (yes in RE2 compat mode) |
| conditionals `(?(expr)yes\|no)` | no | yes |

## RE2 compatibility mode
The default behavior of `regexp2` is to match the .NET regexp engine, however the `RE2` option is provided to change the parsing to increase compatibility with RE2.  Using the `RE2` option when compiling a regexp will not take away any features, but will change the following behaviors:
* add support for named ascii character classes (e.g. `[[:foo:]]`)
* add support for python-style capture groups (e.g. `(P<name>re)`)
* change singleline behavior for `$` to only match end of string (like RE2) (see [#24](https://github.com/dlclark/regexp2/issues/24))
* change the character classes `\d` `\s` and `\w` to match the same characters as RE2. NOTE: if you also use the `ECMAScript` option then this will change the `\s` character class to match ECMAScript instead of RE2.  ECMAScript allows more whitespace characters in `\s` than RE2 (but still fewer than the the default behavior).
* allow character escape sequences to have defaults. For example, by default `\_` isn't a known character escape and will fail to compile, but in RE2 mode it will match the literal character `_`
 
```go
re := regexp2.MustCompile(`Your RE2-compatible pattern`, regexp2.RE2)
if isMatch, _ := re.MatchString(`Something to match`); isMatch {
    //do something
}
```

This feature is a work in progress and I'm open to ideas for more things to put here (maybe more relaxed character escaping rules?).

## Catastrophic Backtracking and Timeouts

`regexp2` supports features that can lead to catastrophic backtracking.
`Regexp.MatchTimeout` can be set to to limit the impact of such behavior; the
match will fail with an error after approximately MatchTimeout. No timeout
checks are done by default.

Timeout checking is not free. The current timeout checking implementation starts
a background worker that updates a clock value approximately once every 100
milliseconds. The matching code compares this value against the precomputed
deadline for the match. The performance impact is as follows.

1.  A match with a timeout runs almost as fast as a match without a timeout.
2.  If any live matches have a timeout, there will be a background CPU load
    (`~0.15%` currently on a modern machine). This load will remain constant
    regardless of the number of matches done including matches done in parallel.
3.  If no live matches are using a timeout, the background load will remain
    until the longest deadline (match timeout + the time when the match started)
    is reached. E.g., if you set a timeout of one minute the load will persist
    for approximately a minute even if the match finishes quickly.

See [PR #58](https://github.com/dlclark/regexp2/pull/58) for more details and 
alternatives considered.

## Goroutine leak error
If you're using a library during unit tests (e.g. https://github.com/uber-go/goleak) that validates all goroutines are exited then you'll likely get an error if you or any of your dependencies use regex's with a MatchTimeout. 
To remedy the problem you'll need to tell the unit test to wait until the backgroup timeout goroutine is exited.

```go
func TestSomething(t *testing.T) {
    defer goleak.VerifyNone(t)
    defer regexp2.StopTimeoutClock()

    // ... test
}

//or

func TestMain(m *testing.M) {
    // setup
    // ...

    // run 
    m.Run()

    //tear down
    regexp2.StopTimeoutClock()
    goleak.VerifyNone(t)
}
```

This will add ~100ms runtime to each test (or TestMain). If that's too much time you can set the clock cycle rate of the timeout goroutine in an init function in a test file. `regexp2.SetTimeoutCheckPeriod` isn't threadsafe so it must be setup before starting any regex's with Timeouts.

```go
func init() {
	//speed up testing by making the timeout clock 1ms
	regexp2.SetTimeoutCheckPeriod(time.Millisecond)
}
```

## ECMAScript compatibility mode
In this mode the engine provides compatibility with the [regex engine](https://tc39.es/ecma262/multipage/text-processing.html#sec-regexp-regular-expression-objects) described in the ECMAScript specification.

Additionally a Unicode mode is provided which allows parsing of `\u{CodePoint}` syntax that is only when both are provided.

## Library features that I'm still working on
- Regex split

## Potential bugs
I've run a battery of tests against regexp2 from various sources and found the debug output matches the .NET engine, but .NET and Go handle strings very differently.  I've attempted to handle these differences, but most of my testing deals with basic ASCII with a little bit of multi-byte Unicode.  There's a chance that there are bugs in the string handling related to character sets with supplementary Unicode chars.  Right-to-Left support is coded, but not well tested either.

## Find a bug?
I'm open to new issues and pull requests with tests if you find something odd!
