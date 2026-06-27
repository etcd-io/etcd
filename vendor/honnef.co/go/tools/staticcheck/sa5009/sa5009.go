package sa5009

import (
	"fmt"
	"go/constant"
	"go/types"

	"honnef.co/go/tools/analysis/callcheck"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"
	"honnef.co/go/tools/knowledge"
	"honnef.co/go/tools/printf"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA5009",
		Requires: []*analysis.Analyzer{buildir.Analyzer},
		Run:      callcheck.Analyzer(rules),
	},
	Doc: &lint.RawDocumentation{
		Title:    `Invalid Printf call`,
		Since:    "2019.2",
		Severity: lint.SeverityError,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

// TODO(dh): detect printf wrappers
var rules = map[string]callcheck.Check{
	"fmt.Errorf":                  func(call *callcheck.Call) { check(call, 0, 1) },
	"fmt.Printf":                  func(call *callcheck.Call) { check(call, 0, 1) },
	"fmt.Sprintf":                 func(call *callcheck.Call) { check(call, 0, 1) },
	"fmt.Fprintf":                 func(call *callcheck.Call) { check(call, 1, 2) },
	"golang.org/x/xerrors.Errorf": func(call *callcheck.Call) { check(call, 0, 1) },
}

type verbFlag int

const (
	isInt verbFlag = 1 << iota
	isBool
	isFP
	isString
	isPointer
	// Verbs that accept "pseudo pointers" will sometimes dereference
	// non-nil pointers. For example, %x on a non-nil *struct will print the
	// individual fields, but on a nil pointer it will print the address.
	isPseudoPointer
	isSlice
	isAny
	noRecurse
)

var verbs = [...]verbFlag{
	'b': isPseudoPointer | isInt | isFP,
	'c': isInt,
	'd': isPseudoPointer | isInt,
	'e': isFP,
	'E': isFP,
	'f': isFP,
	'F': isFP,
	'g': isFP,
	'G': isFP,
	'o': isPseudoPointer | isInt,
	'O': isPseudoPointer | isInt,
	'p': isSlice | isPointer | noRecurse,
	'q': isInt | isString,
	's': isString,
	't': isBool,
	'T': isAny,
	'U': isInt,
	'v': isAny,
	'X': isPseudoPointer | isInt | isFP | isString,
	'x': isPseudoPointer | isInt | isFP | isString,
}

func check(call *callcheck.Call, fIdx, vIdx int) {
	f := call.Args[fIdx]
	var args []ir.Value
	switch v := call.Args[vIdx].Value.Value.(type) {
	case *ir.Slice:
		var ok bool
		args, ok = irutil.Vararg(v)
		if !ok {
			// We don't know what the actual arguments to the function are
			return
		}
	case *ir.Const:
		// nil, i.e. no arguments
	default:
		// We don't know what the actual arguments to the function are
		return
	}
	checkImpl(f, f.Value.Value, args)
}

func checkImpl(carg *callcheck.Argument, f ir.Value, args []ir.Value) {
	var msCache *typeutil.MethodSetCache
	if f.Parent() != nil {
		msCache = &f.Parent().Prog.MethodSets
	}

	elem := func(T types.Type, verb rune) ([]types.Type, bool) {
		if verbs[verb]&noRecurse != 0 {
			return []types.Type{T}, false
		}
		switch T := T.(type) {
		case *types.Slice:
			if verbs[verb]&isSlice != 0 {
				return []types.Type{T}, false
			}
			if verbs[verb]&isString != 0 && types.Identical(T.Elem().Underlying(), types.Typ[types.Byte]) {
				return []types.Type{T}, false
			}
			return []types.Type{T.Elem()}, true
		case *types.Map:
			key := T.Key()
			val := T.Elem()
			return []types.Type{key, val}, true
		case *types.Struct:
			out := make([]types.Type, 0, T.NumFields())
			for field := range T.Fields() {
				out = append(out, field.Type())
			}
			return out, true
		case *types.Array:
			return []types.Type{T.Elem()}, true
		default:
			return []types.Type{T}, false
		}
	}
	isInfo := func(T types.Type, info types.BasicInfo) bool {
		basic, ok := T.Underlying().(*types.Basic)
		return ok && basic.Info()&info != 0
	}

	isFormatter := func(T types.Type, ms *types.MethodSet) bool {
		sel := ms.Lookup(nil, "Format")
		if sel == nil {
			return false
		}
		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			// should be unreachable
			return false
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 2 {
			return false
		}
		// TODO(dh): check the types of the arguments for more
		// precision
		if sig.Results().Len() != 0 {
			return false
		}
		return true
	}

	var seen typeutil.Map[struct{}]
	var checkType func(verb rune, T types.Type, top bool) bool
	checkType = func(verb rune, T types.Type, top bool) bool {
		if top {
			seen = typeutil.Map[struct{}]{}
		}
		if _, ok := seen.At(T); ok {
			return true
		}
		seen.Set(T, struct{}{})
		if int(verb) >= len(verbs) {
			// Unknown verb
			return true
		}

		flags := verbs[verb]
		if flags == 0 {
			// Unknown verb
			return true
		}

		ms := msCache.MethodSet(T)
		if isFormatter(T, ms) {
			// the value is responsible for formatting itself
			return true
		}

		if flags&isString != 0 && (types.Implements(T, knowledge.Interfaces["fmt.Stringer"]) || types.Implements(T, knowledge.Interfaces["error"])) {
			// Check for stringer early because we're about to dereference
			return true
		}

		T = T.Underlying()
		if flags&(isPointer|isPseudoPointer) == 0 && top {
			T = typeutil.Dereference(T)
		}
		if flags&isPseudoPointer != 0 && top {
			t := typeutil.Dereference(T)
			if _, ok := t.Underlying().(*types.Struct); ok {
				T = t
			}
		}

		if _, ok := T.(*types.Interface); ok {
			// We don't know what's in the interface
			return true
		}

		var info types.BasicInfo
		if flags&isInt != 0 {
			info |= types.IsInteger
		}
		if flags&isBool != 0 {
			info |= types.IsBoolean
		}
		if flags&isFP != 0 {
			info |= types.IsFloat | types.IsComplex
		}
		if flags&isString != 0 {
			info |= types.IsString
		}

		if info != 0 && isInfo(T, info) {
			return true
		}

		if flags&isString != 0 {
			isStringyElem := func(typ types.Type) bool {
				if typ, ok := typ.Underlying().(*types.Basic); ok {
					return typ.Kind() == types.Byte
				}
				return false
			}
			switch T := T.(type) {
			case *types.Slice:
				if isStringyElem(T.Elem()) {
					return true
				}
			case *types.Array:
				if isStringyElem(T.Elem()) {
					return true
				}
			}
			if types.Implements(T, knowledge.Interfaces["fmt.Stringer"]) || types.Implements(T, knowledge.Interfaces["error"]) {
				return true
			}
		}

		if flags&isPointer != 0 && typeutil.IsPointerLike(T) {
			return true
		}
		if flags&isPseudoPointer != 0 {
			switch U := T.Underlying().(type) {
			case *types.Pointer:
				if !top {
					return true
				}

				if _, ok := U.Elem().Underlying().(*types.Struct); !ok {
					// TODO(dh): can this condition ever be false? For
					// *T, if T is a struct, we'll already have
					// dereferenced it, meaning the *types.Pointer
					// branch couldn't have been taken. For T that
					// aren't structs, this condition will always
					// evaluate to true.
					return true
				}
			case *types.Chan, *types.Signature:
				// Channels and functions are always treated as
				// pointers and never recursed into.
				return true
			case *types.Basic:
				if U.Kind() == types.UnsafePointer {
					return true
				}
			case *types.Interface:
				// we will already have bailed if the type is an
				// interface.
				panic("unreachable")
			default:
				// other pointer-like types, such as maps or slices,
				// will be printed element-wise.
			}
		}

		if flags&isSlice != 0 {
			if _, ok := T.(*types.Slice); ok {
				return true
			}
		}

		if flags&isAny != 0 {
			return true
		}

		elems, ok := elem(T.Underlying(), verb)
		if !ok {
			return false
		}
		for _, elem := range elems {
			if !checkType(verb, elem, false) {
				return false
			}
		}

		return true
	}

	k, ok := irutil.Flatten(f).(*ir.Const)
	if !ok {
		return
	}
	actions, err := printf.Parse(constant.StringVal(k.Value))
	if err != nil {
		carg.Invalid("couldn't parse format string")
		return
	}

	ptr := 1
	hasExplicit := false

	checkStar := func(verb printf.Verb, star printf.Argument) bool {
		if star, ok := star.(printf.Star); ok {
			idx := 0
			if star.Index == -1 {
				idx = ptr
				ptr++
			} else {
				hasExplicit = true
				idx = star.Index
				ptr = star.Index + 1
			}
			if idx == 0 {
				carg.Invalid(fmt.Sprintf("Printf format %s reads invalid arg 0; indices are 1-based", verb.Raw))
				return false
			}
			if idx > len(args) {
				carg.Invalid(
					fmt.Sprintf("Printf format %s reads arg #%d, but call has only %d args",
						verb.Raw, idx, len(args)))
				return false
			}
			if arg, ok := args[idx-1].(*ir.MakeInterface); ok {
				if !isInfo(arg.X.Type(), types.IsInteger) {
					carg.Invalid(fmt.Sprintf("Printf format %s reads non-int arg #%d as argument of *", verb.Raw, idx))
				}
			}
		}
		return true
	}

	// We only report one problem per format string. Making a
	// mistake with an index tends to invalidate all future
	// implicit indices.
	for _, action := range actions {
		verb, ok := action.(printf.Verb)
		if !ok {
			continue
		}

		if !checkStar(verb, verb.Width) || !checkStar(verb, verb.Precision) {
			return
		}

		off := ptr
		if verb.Value != -1 {
			hasExplicit = true
			off = verb.Value
		}
		if off > len(args) {
			carg.Invalid(
				fmt.Sprintf("Printf format %s reads arg #%d, but call has only %d args",
					verb.Raw, off, len(args)))
			return
		} else if verb.Value == 0 && verb.Letter != '%' {
			carg.Invalid(fmt.Sprintf("Printf format %s reads invalid arg 0; indices are 1-based", verb.Raw))
			return
		} else if off != 0 {
			arg, ok := args[off-1].(*ir.MakeInterface)
			if ok {
				if !checkType(verb.Letter, arg.X.Type(), true) {
					carg.Invalid(fmt.Sprintf("Printf format %s has arg #%d of wrong type %s",
						verb.Raw, ptr, args[ptr-1].(*ir.MakeInterface).X.Type()))
					return
				}
			}
		}

		switch verb.Value {
		case -1:
			// Consume next argument
			ptr++
		case 0:
			// Don't consume any arguments
		default:
			ptr = verb.Value + 1
		}
	}

	if !hasExplicit && ptr <= len(args) {
		carg.Invalid(fmt.Sprintf("Printf call needs %d args but has %d args", ptr-1, len(args)))
	}
}
