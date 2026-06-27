//go:generate go run ../generate.go

// Package stylecheck contains analyzes that enforce style rules.
// Most of the recommendations made are universally agreed upon by the wider Go community.
// Some analyzes, however, implement stricter rules that not everyone will agree with.
// In the context of Staticcheck, these analyzes are not enabled by default.
//
// For the most part it is recommended to follow the advice given by the analyzers that are enabled by default,
// but you may want to disable additional analyzes on a case by case basis.
package stylecheck
