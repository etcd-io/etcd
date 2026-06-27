// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package printf

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"

	"github.com/Antonboom/testifylint/internal/analysisutil"
)

// CheckPrintf checks a call to a formatted print routine such as Printf.
func CheckPrintf(
	pass *analysis.Pass,
	call *ast.CallExpr,
	fnName string,
	format string,
	formatIdx int,
) {
	firstArg := formatIdx + 1 // Arguments are immediately after format string.
	if !strings.Contains(format, "%") {
		if len(call.Args) > firstArg {
			pass.Reportf(call.Lparen, "%s call has arguments but no formatting directives", fnName)
		}
		return
	}
	// Hard part: check formats against args.
	argNum := firstArg
	maxArgNum := firstArg
	anyIndex := false
	for i, w := 0, 0; i < len(format); i += w {
		w = 1
		if format[i] != '%' {
			continue
		}
		state := parsePrintfVerb(pass, call, fnName, format[i:], firstArg, argNum)
		if state == nil {
			return
		}
		w = len(state.format)
		if !okPrintfArg(pass, call, state) { // One error per format is enough.
			return
		}
		if state.hasIndex {
			anyIndex = true
		}
		if state.verb == 'w' {
			pass.Reportf(call.Pos(), "%s does not support error-wrapping directive %%w", state.name)
			return
		}
		if len(state.argNums) > 0 {
			// Continue with the next sequential argument.
			argNum = state.argNums[len(state.argNums)-1] + 1
		}
		for _, n := range state.argNums {
			if n >= maxArgNum {
				maxArgNum = n + 1
			}
		}
	}
	// Dotdotdot is hard.
	if call.Ellipsis.IsValid() && maxArgNum >= len(call.Args)-1 {
		return
	}
	// If any formats are indexed, extra arguments are ignored.
	if anyIndex {
		return
	}
	// There should be no leftover arguments.
	if maxArgNum != len(call.Args) {
		expect := maxArgNum - firstArg
		numArgs := len(call.Args) - firstArg
		pass.ReportRangef(call, "%s call needs %v but has %v", fnName, count(expect, "arg"), count(numArgs, "arg"))
	}
}

// formatState holds the parsed representation of a printf directive such as "%3.*[4]d".
// It is constructed by parsePrintfVerb.
type formatState struct {
	verb     rune   // the format verb: 'd' for "%d"
	format   string // the full format directive from % through verb, "%.3d".
	name     string // Printf, Sprintf etc.
	flags    []byte // the list of # + etc.
	argNums  []int  // the successive argument numbers that are consumed, adjusted to refer to actual arg in call
	firstArg int    // Index of first argument after the format in the Printf call.
	// Used only during parse.
	pass         *analysis.Pass
	call         *ast.CallExpr
	argNum       int  // Which argument we're expecting to format now.
	hasIndex     bool // Whether the argument is indexed.
	indexPending bool // Whether we have an indexed argument that has not resolved.
	nbytes       int  // number of bytes of the format string consumed.
}

// parseFlags accepts any printf flags.
func (s *formatState) parseFlags() {
	for s.nbytes < len(s.format) {
		switch c := s.format[s.nbytes]; c {
		case '#', '0', '+', '-', ' ':
			s.flags = append(s.flags, c)
			s.nbytes++
		default:
			return
		}
	}
}

// scanNum advances through a decimal number if present.
func (s *formatState) scanNum() {
	for ; s.nbytes < len(s.format); s.nbytes++ {
		c := s.format[s.nbytes]
		if c < '0' || '9' < c {
			return
		}
	}
}

// parseIndex scans an index expression. It returns false if there is a syntax error.
func (s *formatState) parseIndex() bool {
	if s.nbytes == len(s.format) || s.format[s.nbytes] != '[' {
		return true
	}
	// Argument index present.
	s.nbytes++ // skip '['
	start := s.nbytes
	s.scanNum()
	ok := true
	if s.nbytes == len(s.format) || s.nbytes == start || s.format[s.nbytes] != ']' {
		ok = false // syntax error is either missing "]" or invalid index.
		s.nbytes = strings.Index(s.format[start:], "]")
		if s.nbytes < 0 {
			s.pass.ReportRangef(s.call, "%s format %s is missing closing ]", s.name, s.format)
			return false
		}
		s.nbytes += start
	}
	arg32, err := strconv.ParseInt(s.format[start:s.nbytes], 10, 32)
	if err != nil || !ok || arg32 <= 0 || arg32 > int64(len(s.call.Args)-s.firstArg) {
		s.pass.ReportRangef(s.call, "%s format has invalid argument index [%s]", s.name, s.format[start:s.nbytes])
		return false
	}
	s.nbytes++ // skip ']'
	arg := int(arg32)
	arg += s.firstArg - 1 // We want to zero-index the actual arguments.
	s.argNum = arg
	s.hasIndex = true
	s.indexPending = true
	return true
}

// parseNum scans a width or precision (or *). It returns false if there's a bad index expression.
func (s *formatState) parseNum() bool {
	if s.nbytes < len(s.format) && s.format[s.nbytes] == '*' {
		if s.indexPending { // Absorb it.
			s.indexPending = false
		}
		s.nbytes++
		s.argNums = append(s.argNums, s.argNum)
		s.argNum++
	} else {
		s.scanNum()
	}
	return true
}

// parsePrecision scans for a precision. It returns false if there's a bad index expression.
func (s *formatState) parsePrecision() bool {
	// If there's a period, there may be a precision.
	if s.nbytes < len(s.format) && s.format[s.nbytes] == '.' {
		s.flags = append(s.flags, '.') // Treat precision as a flag.
		s.nbytes++
		if !s.parseIndex() {
			return false
		}
		if !s.parseNum() {
			return false
		}
	}
	return true
}

// isFormatter reports whether t could satisfy fmt.Formatter.
// The only interface method to look for is "Format(State, rune)".
func isFormatter(typ types.Type) bool {
	// If the type is an interface, the value it holds might satisfy fmt.Formatter.
	if _, ok := typ.Underlying().(*types.Interface); ok {
		// Don't assume type parameters could be formatters. With the greater
		// expressiveness of constraint interface syntax we expect more type safety
		// when using type parameters.
		if !isTypeParam(typ) {
			return true
		}
	}
	obj, _, _ := types.LookupFieldOrMethod(typ, false, nil, "Format")
	fn, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Type().(*types.Signature)
	return sig.Params().Len() == 2 &&
		sig.Results().Len() == 0 &&
		isNamedType(sig.Params().At(0).Type(), "fmt", "State") &&
		types.Identical(sig.Params().At(1).Type(), types.Typ[types.Rune])
}

// isTypeParam reports whether t is a type parameter (or an alias of one).
func isTypeParam(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}

// isNamedType reports whether t is the named type with the given package path
// and one of the given names.
// This function avoids allocating the concatenation of "pkg.Name",
// which is important for the performance of syntax matching.
func isNamedType(t types.Type, pkgPath string, names ...string) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	obj := n.Obj()
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != pkgPath {
		return false
	}
	name := obj.Name()
	for _, n := range names {
		if name == n {
			return true
		}
	}
	return false
}

// parsePrintfVerb looks the formatting directive that begins the format string
// and returns a formatState that encodes what the directive wants, without looking
// at the actual arguments present in the call. The result is nil if there is an error.
func parsePrintfVerb(pass *analysis.Pass, call *ast.CallExpr, name, format string, firstArg, argNum int) *formatState {
	state := &formatState{
		format:   format,
		name:     name,
		flags:    make([]byte, 0, 5),
		argNum:   argNum,
		argNums:  make([]int, 0, 1),
		nbytes:   1, // There's guaranteed to be a percent sign.
		firstArg: firstArg,
		pass:     pass,
		call:     call,
	}
	// There may be flags.
	state.parseFlags()
	// There may be an index.
	if !state.parseIndex() {
		return nil
	}
	// There may be a width.
	if !state.parseNum() {
		return nil
	}
	// There may be a precision.
	if !state.parsePrecision() {
		return nil
	}
	// Now a verb, possibly prefixed by an index (which we may already have).
	if !state.indexPending && !state.parseIndex() {
		return nil
	}
	if state.nbytes == len(state.format) {
		pass.ReportRangef(call.Fun, "%s format %s is missing verb at end of string", name, state.format)
		return nil
	}
	verb, w := utf8.DecodeRuneInString(state.format[state.nbytes:])
	state.verb = verb
	state.nbytes += w
	if verb != '%' {
		state.argNums = append(state.argNums, state.argNum)
	}
	state.format = state.format[:state.nbytes]
	return state
}

// printfArgType encodes the types of expressions a printf verb accepts. It is a bitmask.
type printfArgType int

const (
	argBool printfArgType = 1 << iota
	argInt
	argRune
	argString
	argFloat
	argComplex
	argPointer
	argError
	anyType printfArgType = ^0
)

type printVerb struct {
	verb  rune   // User may provide verb through Formatter; could be a rune.
	flags string // known flags are all ASCII
	typ   printfArgType
}

// Common flag sets for printf verbs.
const (
	noFlag       = ""
	numFlag      = " -+.0"
	sharpNumFlag = " -+.0#"
	allFlags     = " -+.0#"
)

// printVerbs identifies which flags are known to printf for each verb.
var printVerbs = []printVerb{
	// '-' is a width modifier, always valid.
	// '.' is a precision for float, max width for strings.
	// '+' is required sign for numbers, Go format for %v.
	// '#' is alternate format for several verbs.
	// ' ' is spacer for numbers
	{'%', noFlag, 0},
	{'b', sharpNumFlag, argInt | argFloat | argComplex | argPointer},
	{'c', "-", argRune | argInt},
	{'d', numFlag, argInt | argPointer},
	{'e', sharpNumFlag, argFloat | argComplex},
	{'E', sharpNumFlag, argFloat | argComplex},
	{'f', sharpNumFlag, argFloat | argComplex},
	{'F', sharpNumFlag, argFloat | argComplex},
	{'g', sharpNumFlag, argFloat | argComplex},
	{'G', sharpNumFlag, argFloat | argComplex},
	{'o', sharpNumFlag, argInt | argPointer},
	{'O', sharpNumFlag, argInt | argPointer},
	{'p', "-#", argPointer},
	{'q', " -+.0#", argRune | argInt | argString},
	{'s', " -+.0", argString},
	{'t', "-", argBool},
	{'T', "-", anyType},
	{'U', "-#", argRune | argInt},
	{'v', allFlags, anyType},
	{'w', allFlags, argError},
	{'x', sharpNumFlag, argRune | argInt | argString | argPointer | argFloat | argComplex},
	{'X', sharpNumFlag, argRune | argInt | argString | argPointer | argFloat | argComplex},
}

// okPrintfArg compares the formatState to the arguments actually present,
// reporting any discrepancies it can discern. If the final argument is ellipsissed,
// there's little it can do for that.
func okPrintfArg(pass *analysis.Pass, call *ast.CallExpr, state *formatState) (ok bool) {
	var v printVerb
	found := false
	// Linear scan is fast enough for a small list.
	for _, v = range printVerbs {
		if v.verb == state.verb {
			found = true
			break
		}
	}

	// Could current arg implement fmt.Formatter?
	// Skip check for the %w verb, which requires an error.
	formatter := false
	if v.typ != argError && state.argNum < len(call.Args) {
		if tv, ok := pass.TypesInfo.Types[call.Args[state.argNum]]; ok {
			formatter = isFormatter(tv.Type)
		}
	}

	if !formatter {
		if !found {
			pass.ReportRangef(call, "%s format %s has unknown verb %c", state.name, state.format, state.verb)
			return false
		}
		for _, flag := range state.flags {
			// TODO: Disable complaint about '0' for Go 1.10. To be fixed properly in 1.11.
			// See issues 23598 and 23605.
			if flag == '0' {
				continue
			}
			if !strings.ContainsRune(v.flags, rune(flag)) {
				pass.ReportRangef(call, "%s format %s has unrecognized flag %c", state.name, state.format, flag)
				return false
			}
		}
	}
	// Verb is good. If len(state.argNums)>trueArgs, we have something like %.*s and all
	// but the final arg must be an integer.
	trueArgs := 1
	if state.verb == '%' {
		trueArgs = 0
	}
	nargs := len(state.argNums)
	for i := 0; i < nargs-trueArgs; i++ {
		if !argCanBeChecked(pass, call, i, state) {
			return
		}
		// NOTE(a.telyshev): `matchArgType` leads to a lot of "golang.org/x/tools/internal" code.
		/*
			argNum := state.argNums[i]
			arg := call.Args[argNum]

			if reason, ok := matchArgType(pass, argInt, arg); !ok {
				details := ""
				if reason != "" {
					details = " (" + reason + ")"
				}
				pass.ReportRangef(call, "%s format %s uses non-int %s%s as argument of *", state.name, state.format, analysisutil.Format(pass.Fset, arg), details)
				return false
			}
		*/
	}

	if state.verb == '%' || formatter {
		return true
	}
	argNum := state.argNums[len(state.argNums)-1]
	if !argCanBeChecked(pass, call, len(state.argNums)-1, state) {
		return false
	}
	arg := call.Args[argNum]
	if isFunctionValue(pass, arg) && state.verb != 'p' && state.verb != 'T' {
		pass.ReportRangef(call, "%s format %s arg %s is a func value, not called", state.name, state.format, analysisutil.NodeString(pass.Fset, arg))
		return false
	}
	// NOTE(a.telyshev): `matchArgType` leads to a lot of "golang.org/x/tools/internal" code.
	/*
		if reason, ok := matchArgType(pass, v.typ, arg); !ok {
			typeString := ""
			if typ := pass.TypesInfo.Types[arg].Type; typ != nil {
				typeString = typ.String()
			}
			details := ""
			if reason != "" {
				details = " (" + reason + ")"
			}
			pass.ReportRangef(call, "%s format %s has arg %s of wrong type %s%s", state.name, state.format, analysisutil.Format(pass.Fset, arg), typeString, details)
			return false
		}
	*/
	if v.typ&argString != 0 && v.verb != 'T' && !bytes.Contains(state.flags, []byte{'#'}) {
		if methodName, ok := recursiveStringer(pass, arg); ok {
			pass.ReportRangef(call, "%s format %s with arg %s causes recursive %s method call", state.name, state.format, analysisutil.NodeString(pass.Fset, arg), methodName)
			return false
		}
	}
	return true
}

// recursiveStringer reports whether the argument e is a potential
// recursive call to stringer or is an error, such as t and &t in these examples:
//
//	func (t *T) String() string { printf("%s",  t) }
//	func (t  T) Error() string { printf("%s",  t) }
//	func (t  T) String() string { printf("%s", &t) }
func recursiveStringer(pass *analysis.Pass, e ast.Expr) (string, bool) {
	typ := pass.TypesInfo.Types[e].Type

	// It's unlikely to be a recursive stringer if it has a Format method.
	if isFormatter(typ) {
		return "", false
	}

	// Does e allow e.String() or e.Error()?
	strObj, _, _ := types.LookupFieldOrMethod(typ, false, pass.Pkg, "String")
	strMethod, strOk := strObj.(*types.Func)
	errObj, _, _ := types.LookupFieldOrMethod(typ, false, pass.Pkg, "Error")
	errMethod, errOk := errObj.(*types.Func)
	if !strOk && !errOk {
		return "", false
	}

	// inScope returns true if e is in the scope of f.
	inScope := func(e ast.Expr, f *types.Func) bool {
		return f.Scope() != nil && f.Scope().Contains(e.Pos())
	}

	// Is the expression e within the body of that String or Error method?
	var method *types.Func
	if strOk && strMethod.Pkg() == pass.Pkg && inScope(e, strMethod) {
		method = strMethod
	} else if errOk && errMethod.Pkg() == pass.Pkg && inScope(e, errMethod) {
		method = errMethod
	} else {
		return "", false
	}

	sig := method.Type().(*types.Signature)
	if !isStringer(sig) {
		return "", false
	}

	// Is it the receiver r, or &r?
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X // strip off & from &r
	}
	if id, ok := e.(*ast.Ident); ok {
		if pass.TypesInfo.Uses[id] == sig.Recv() {
			return method.FullName(), true
		}
	}
	return "", false
}

// isStringer reports whether the method signature matches the String() definition in fmt.Stringer.
func isStringer(sig *types.Signature) bool {
	return sig.Params().Len() == 0 &&
		sig.Results().Len() == 1 &&
		sig.Results().At(0).Type() == types.Typ[types.String]
}

// isFunctionValue reports whether the expression is a function as opposed to a function call.
// It is almost always a mistake to print a function value.
func isFunctionValue(pass *analysis.Pass, e ast.Expr) bool {
	if typ := pass.TypesInfo.Types[e].Type; typ != nil {
		// Don't call Underlying: a named func type with a String method is ok.
		// TODO(adonovan): it would be more precise to check isStringer.
		_, ok := typ.(*types.Signature)
		return ok
	}
	return false
}

// argCanBeChecked reports whether the specified argument is statically present;
// it may be beyond the list of arguments or in a terminal slice... argument, which
// means we can't see it.
func argCanBeChecked(pass *analysis.Pass, call *ast.CallExpr, formatArg int, state *formatState) bool {
	argNum := state.argNums[formatArg]
	if argNum <= 0 {
		return false
	}
	if argNum < len(call.Args)-1 {
		return true // Always OK.
	}
	if call.Ellipsis.IsValid() {
		return false // We just can't tell; there could be many more arguments.
	}
	if argNum < len(call.Args) {
		return true
	}
	// There are bad indexes in the format or there are fewer arguments than the format needs.
	// This is the argument number relative to the format: Printf("%s", "hi") will give 1 for the "hi".
	arg := argNum - state.firstArg + 1 // People think of arguments as 1-indexed.
	pass.ReportRangef(call, "%s format %s reads arg #%d, but call has %v", state.name, state.format, arg, count(len(call.Args)-state.firstArg, "arg"))
	return false
}

// count(n, what) returns "1 what" or "N whats"
// (assuming the plural of what is whats).
func count(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}
