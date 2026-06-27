//go:generate go run ../generate.go

// Package quickfix contains analyzes that implement code refactorings.
// None of these analyzers produce diagnostics that have to be followed.
// Most of the time, they only provide alternative ways of doing things,
// requiring users to make informed decisions.
//
// None of these analyzes should fail a build, and they are likely useless in CI as a whole.
package quickfix
