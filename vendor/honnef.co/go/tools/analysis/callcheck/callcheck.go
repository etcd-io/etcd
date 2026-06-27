// Package callcheck provides a framework for validating arguments in function calls.
package callcheck

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/ir/irutil"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"
)

type Call struct {
	Pass  *analysis.Pass
	Instr ir.CallInstruction
	Args  []*Argument

	Parent *ir.Function

	invalids []string
}

func (c *Call) Invalid(msg string) {
	c.invalids = append(c.invalids, msg)
}

type Argument struct {
	Value    Value
	invalids []string
}

type Value struct {
	Value ir.Value
}

func (arg *Argument) Invalid(msg string) {
	arg.invalids = append(arg.invalids, msg)
}

type Check func(call *Call)

func Analyzer(rules map[string]Check) func(pass *analysis.Pass) (any, error) {
	return func(pass *analysis.Pass) (any, error) {
		return checkCalls(pass, rules)
	}
}

func checkCalls(pass *analysis.Pass, rules map[string]Check) (any, error) {
	cb := func(caller *ir.Function, site ir.CallInstruction, callee *ir.Function) {
		obj, ok := callee.Object().(*types.Func)
		if !ok {
			return
		}

		r, ok := rules[typeutil.FuncName(obj)]
		if !ok {
			return
		}
		var args []*Argument
		irargs := site.Common().Args
		if callee.Signature.Recv() != nil {
			irargs = irargs[1:]
		}
		for _, arg := range irargs {
			if iarg, ok := arg.(*ir.MakeInterface); ok {
				arg = iarg.X
			}
			args = append(args, &Argument{Value: Value{arg}})
		}
		call := &Call{
			Pass:   pass,
			Instr:  site,
			Args:   args,
			Parent: site.Parent(),
		}
		r(call)

		var astcall *ast.CallExpr
		switch source := site.Source().(type) {
		case *ast.CallExpr:
			astcall = source
		case *ast.DeferStmt:
			astcall = source.Call
		case *ast.GoStmt:
			astcall = source.Call
		case nil:
			// TODO(dh): I am not sure this can actually happen. If it
			// can't, we should remove this case, and also stop
			// checking for astcall == nil in the code that follows.
		default:
			panic(fmt.Sprintf("unhandled case %T", source))
		}

		for idx, arg := range call.Args {
			for _, e := range arg.invalids {
				if astcall != nil {
					if idx < len(astcall.Args) {
						report.Report(pass, astcall.Args[idx], e)
					} else {
						// this is an instance of fn1(fn2()) where fn2
						// returns multiple values. Report the error
						// at the next-best position that we have, the
						// first argument. An example of a check that
						// triggers this is checkEncodingBinaryRules.
						report.Report(pass, astcall.Args[0], e)
					}
				} else {
					report.Report(pass, site, e)
				}
			}
		}
		for _, e := range call.invalids {
			report.Report(pass, call.Instr, e)
		}
	}
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		eachCall(fn, cb)
	}
	return nil, nil
}

func eachCall(fn *ir.Function, cb func(caller *ir.Function, site ir.CallInstruction, callee *ir.Function)) {
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if site, ok := instr.(ir.CallInstruction); ok {
				if g := site.Common().StaticCallee(); g != nil {
					cb(fn, site, g)
				}
			}
		}
	}
}

func ExtractConstExpectKind(v Value, kind constant.Kind) *ir.Const {
	k := extractConst(v.Value)
	if k == nil || k.Value == nil || k.Value.Kind() != kind {
		return nil
	}
	return k
}

func ExtractConst(v Value) *ir.Const {
	return extractConst(v.Value)
}

func extractConst(v ir.Value) *ir.Const {
	v = irutil.Flatten(v)
	switch v := v.(type) {
	case *ir.Const:
		return v
	case *ir.MakeInterface:
		return extractConst(v.X)
	default:
		return nil
	}
}
