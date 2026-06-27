// Package analysisutil contains functions common for `analyzer` and `internal/checkers` packages.
// In addition, it is intended to "lighten" these packages.
//
// If the function is common to several packages, or it makes sense to test it separately without
// "polluting" the target package with tests of private functionality, then you can put function in this package.
//
// It's important to avoid dependency on `golang.org/x/tools/go/analysis` in the helpers API.
// This makes the API "narrower" and also allows you to test functions without some "abstraction leaks".
package analysisutil
