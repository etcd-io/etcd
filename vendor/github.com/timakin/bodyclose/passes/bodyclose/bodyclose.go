package bodyclose

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"github.com/gostaticanalysis/analysisutil"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

var Analyzer = &analysis.Analyzer{
	Name: "bodyclose",
	Doc:  Doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		buildssa.Analyzer,
	},
}

func init() {
	Analyzer.Flags.BoolVar(&checkConsumptionFlag, "check-consumption", false, "also check that response body is consumed")
}

var checkConsumptionFlag bool

const (
	Doc = "checks whether HTTP response body is closed successfully"

	nethttpPath = "net/http"
	closeMethod = "Close"
)

type runner struct {
	pass             *analysis.Pass
	resObj           types.Object
	resTyp           *types.Pointer
	bodyObj          types.Object
	closeMthd        *types.Func
	skipFile         map[*ast.File]bool
	checkConsumption bool
}

// run executes an analysis for the pass
func run(pass *analysis.Pass) (interface{}, error) {
	r := runner{
		pass:             pass,
		checkConsumption: checkConsumptionFlag,
	}
	funcs := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA).SrcFuncs

	r.resObj = analysisutil.LookupFromImports(pass.Pkg.Imports(), nethttpPath, "Response")
	if r.resObj == nil {
		// skip checking
		return nil, nil
	}

	resNamed, ok := r.resObj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("cannot find http.Response")
	}
	r.resTyp = types.NewPointer(resNamed)

	resStruct, ok := r.resObj.Type().Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Errorf("cannot find http.Response")
	}
	for i := 0; i < resStruct.NumFields(); i++ {
		field := resStruct.Field(i)
		if field.Id() == "Body" {
			r.bodyObj = field

			break
		}
	}
	if r.bodyObj == nil {
		return nil, fmt.Errorf("cannot find the object http.Response.Body")
	}
	bodyNamed := r.bodyObj.Type().(*types.Named)
	bodyItrf := bodyNamed.Underlying().(*types.Interface)
	for i := 0; i < bodyItrf.NumMethods(); i++ {
		bmthd := bodyItrf.Method(i)
		if bmthd.Id() == closeMethod {
			r.closeMthd = bmthd

			break
		}
	}

	r.skipFile = map[*ast.File]bool{}
FuncLoop:
	for _, f := range funcs {
		// skip if the function is just referenced
		for i := 0; i < f.Signature.Results().Len(); i++ {
			if f.Signature.Results().At(i).Type().String() == r.resTyp.String() {
				continue FuncLoop
			}
		}

		for _, b := range f.Blocks {
			for i := range b.Instrs {
				pos := b.Instrs[i].Pos()
				if r.isopen(b, i) {
					if r.checkConsumption {
						pass.Reportf(pos, "response body must be closed and consumed")
					} else {
						pass.Reportf(pos, "response body must be closed")
					}
				}
			}
		}
	}

	return nil, nil
}

func (r *runner) isopen(b *ssa.BasicBlock, i int) bool {
	call, ok := r.getReqCall(b.Instrs[i])
	if !ok {
		return false
	}

	if len(*call.Referrers()) == 0 {
		return true
	}

	if instr, ok := b.Instrs[i].(*ssa.Call); ok {
		//  httptest.ResponseRecorder is not needed closing the response body because no-op.
		if callee := instr.Call.StaticCallee(); callee != nil && callee.Name() == "Result" {
			if callee.Pkg != nil && callee.Pkg.Pkg.Name() == "httptest" {
				if recv := callee.Signature.Recv(); recv != nil && recv.Type().String() == "*net/http/httptest.ResponseRecorder" {
					return false
				}
			}
		}
	}

	cRefs := *call.Referrers()
	for _, cRef := range cRefs {
		val, ok := r.getResVal(cRef)
		if !ok {
			continue
		}

		if len(*val.Referrers()) == 0 {
			return true
		}
		resRefs := *val.Referrers()
		for _, resRef := range resRefs {
			switch resRef := resRef.(type) {
			case *ssa.Store: // Call in Closure function / Response is global variable
				if _, ok := resRef.Addr.(*ssa.Global); ok {
					// Referrers for globals are always nil, so skip.
					return false
				}

				if len(*resRef.Addr.Referrers()) == 0 {
					return true
				}

				for _, aref := range *resRef.Addr.Referrers() {
					if c, ok := aref.(*ssa.MakeClosure); ok {
						f := c.Fn.(*ssa.Function)
						if r.noImportedNetHTTP(f) {
							// skip this
							return false
						}
						called := r.isClosureCalled(c)

						return r.calledInFunc(f, called)
					}

					// Case when calling Close() from struct field or method
					if s, ok := aref.(*ssa.Store); ok {
						if f, ok := s.Addr.(*ssa.FieldAddr); ok {
							for _, bRef := range f.Block().Instrs {
								bOp, ok := r.getBodyOp(bRef)
								if !ok {
									continue
								}
								for _, ccall := range *bOp.Referrers() {
									if r.isCloseCall(ccall) {
										return false
									}
								}
							}
						}
					}
				}
			case *ssa.Call, *ssa.Defer: // Indirect function call
				// Hacky way to extract CommonCall
				var call ssa.CallCommon
				switch rr := resRef.(type) {
				case *ssa.Call:
					call = rr.Call
				case *ssa.Defer:
					call = rr.Call
				}

				if f, ok := call.Value.(*ssa.Function); ok {
					for _, b := range f.Blocks {
						for i, bi := range b.Instrs {
							if r.isCloseCall(bi) {
								return false
							}

							if r.isopen(b, i) {
								return true
							}
						}
					}
				}
			case *ssa.FieldAddr: // Normal reference to response entity
				if resRef.Referrers() == nil {
					return true
				}

				bRefs := *resRef.Referrers()

				for _, bRef := range bRefs {
					bOp, ok := r.getBodyOp(bRef)
					if !ok {
						continue
					}
					if len(*bOp.Referrers()) == 0 {
						return true
					}
					if r.isBodyProperlyHandled(bOp) {
						return false
					}
				}
			case *ssa.Phi: // Called in the higher-level block
				if resRef.Referrers() == nil {
					return true
				}

				bRefs := *resRef.Referrers()

				for _, bRef := range bRefs {
					switch instr := bRef.(type) {
					case *ssa.FieldAddr:
						bRefs := *instr.Referrers()
						for _, bRef := range bRefs {
							bOp, ok := r.getBodyOp(bRef)
							if !ok {
								continue
							}
							if len(*bOp.Referrers()) == 0 {
								return true
							}
							if r.isBodyProperlyHandled(bOp) {
								return false
							}
						}
					}
				}
			}
		}
	}

	return true
}

func (r *runner) getReqCall(instr ssa.Instruction) (*ssa.Call, bool) {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return nil, false
	}
	callType := call.Type().String()
	if !strings.Contains(callType, r.resTyp.String()) ||
		strings.Contains(callType, "net/http.ResponseController") {
		return nil, false
	}

	return call, true
}

func (r *runner) getResVal(instr ssa.Instruction) (ssa.Value, bool) {
	switch instr := instr.(type) {
	case *ssa.FieldAddr:
		if instr.X.Type().String() == r.resTyp.String() {
			return instr.X.(ssa.Value), true
		}
	case ssa.Value:
		if instr.Type().String() == r.resTyp.String() {
			return instr, true
		}
	case *ssa.Store:
		if instr.Val.Type().String() == r.resTyp.String() {
			return instr.Val, true
		}
	}
	return nil, false
}

func (r *runner) getBodyOp(instr ssa.Instruction) (*ssa.UnOp, bool) {
	op, ok := instr.(*ssa.UnOp)
	if !ok {
		return nil, false
	}
	if op.Type() != r.bodyObj.Type() {
		return nil, false
	}
	return op, true
}

// isBodyProperlyHandled checks if response body is properly handled (closed and optionally consumed based on flag)
func (r *runner) isBodyProperlyHandled(bOp *ssa.UnOp) bool {
	ccalls := *bOp.Referrers()

	for _, ccall := range ccalls {
		if r.isCloseCall(ccall) {
			// Early return if consumption checking is disabled
			if !r.checkConsumption {
				return true
			}
			// Close found and consumption checking enabled - check consumption
			return r.hasConsumptionForBody(bOp)
		}
	}

	// No close call found
	return false
}

// hasConsumptionForBody searches the function for consumption calls that use the specific response body
func (r *runner) hasConsumptionForBody(bodyOp *ssa.UnOp) bool {
	fn := bodyOp.Block().Parent()

	// Search for consumption functions that specifically consume this response body
	for _, block := range fn.Blocks {
		for _, blockInstr := range block.Instrs {
			if call, ok := blockInstr.(*ssa.Call); ok {
				if r.isConsumptionFunction(call) && r.isCallUsingBody(call, bodyOp) {
					return true
				}
			}
		}
	}

	return false
}

// isCallUsingBody checks if a consumption function call uses the specific response body
func (r *runner) isCallUsingBody(call *ssa.Call, responseBodyOp *ssa.UnOp) bool {
	// Get the FieldAddr of the response body we're checking
	responseBodyFieldAddr, ok := responseBodyOp.X.(*ssa.FieldAddr)
	if !ok {
		return false
	}

	// Check if any argument to the call refers to this specific response body
	for _, arg := range call.Call.Args {
		if r.isArgumentMatchingBody(arg, responseBodyFieldAddr) {
			return true
		}
	}

	return false
}

// isArgumentMatchingBody checks if a function argument refers to the same response body instance
func (r *runner) isArgumentMatchingBody(arg ssa.Value, responseBodyFieldAddr *ssa.FieldAddr) bool {
	switch v := arg.(type) {
	case *ssa.FieldAddr:
		// Direct field access - check if it's accessing Body field of same response
		return v.X == responseBodyFieldAddr.X && v.Field == responseBodyFieldAddr.Field
	case *ssa.UnOp:
		// Dereference of field access - check if it's dereferencing the same response body field
		if fieldAddr, ok := v.X.(*ssa.FieldAddr); ok {
			return fieldAddr.X == responseBodyFieldAddr.X && fieldAddr.Field == responseBodyFieldAddr.Field
		}
	case *ssa.ChangeInterface:
		// Type conversion - check if it converts the response body
		if unOp, ok := v.X.(*ssa.UnOp); ok {
			if fieldAddr, ok := unOp.X.(*ssa.FieldAddr); ok {
				return fieldAddr.X == responseBodyFieldAddr.X && fieldAddr.Field == responseBodyFieldAddr.Field
			}
		}
	}
	return false
}

func (r *runner) isConsumptionFunction(call *ssa.Call) bool {
	if call.Call.StaticCallee() != nil {
		callee := call.Call.StaticCallee()
		if callee.Pkg != nil {
			pkg := callee.Pkg.Pkg.Path()
			name := callee.Name()

			// Check for known consumption functions
			if (pkg == "io" && (name == "Copy" || name == "ReadAll")) ||
				(pkg == "io/ioutil" && name == "ReadAll") ||
				(pkg == "encoding/json" && name == "NewDecoder") ||
				(pkg == "bufio" && (name == "NewScanner" || name == "NewReader")) {
				return true
			}
		}
	}
	return false
}

func (r *runner) isCloseCall(ccall ssa.Instruction) bool {
	switch ccall := ccall.(type) {
	case *ssa.Defer:
		if ccall.Call.Method != nil && ccall.Call.Method.Name() == r.closeMthd.Name() {
			return true
		}
	case *ssa.Call:
		if ccall.Call.Method != nil && ccall.Call.Method.Name() == r.closeMthd.Name() {
			return true
		}
	case *ssa.ChangeInterface:
		if ccall.Type().String() == "io.Closer" {
			closeMtd := ccall.Type().Underlying().(*types.Interface).Method(0)
			crs := *ccall.Referrers()
			for _, cs := range crs {
				if cs, ok := cs.(*ssa.Defer); ok {
					if val, ok := cs.Common().Value.(*ssa.Function); ok {
						for _, b := range val.Blocks {
							for _, instr := range b.Instrs {
								if c, ok := instr.(*ssa.Call); ok {
									if c.Call.Method == closeMtd {
										return true
									}
								}
							}
						}
					}
				}

				if returnOp, ok := cs.(*ssa.Return); ok {
					for _, resultValue := range returnOp.Results {
						if resultValue.Type().String() == "io.Closer" {
							return true
						}
					}
				}
			}
		}
	case *ssa.Return:
		for _, resultValue := range ccall.Results {
			if resultValue.Type().String() == "io.ReadCloser" {
				return true
			}
		}
	}
	return false
}

func (r *runner) isClosureCalled(c *ssa.MakeClosure) bool {
	refs := *c.Referrers()
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		switch ref.(type) {
		case *ssa.Call, *ssa.Defer:
			return true
		}
	}
	return false
}

func (r *runner) noImportedNetHTTP(f *ssa.Function) (ret bool) {
	obj := f.Object()
	if obj == nil {
		return false
	}

	file := analysisutil.File(r.pass, obj.Pos())
	if file == nil {
		return false
	}

	if skip, has := r.skipFile[file]; has {
		return skip
	}
	defer func() {
		r.skipFile[file] = ret
	}()

	for _, impt := range file.Imports {
		path, err := strconv.Unquote(impt.Path.Value)
		if err != nil {
			continue
		}
		path = analysisutil.RemoveVendor(path)
		if path == nethttpPath {
			return false
		}
	}

	return true
}

func (r *runner) calledInFunc(f *ssa.Function, called bool) bool {
	for _, b := range f.Blocks {
		for i, instr := range b.Instrs {
			switch instr := instr.(type) {
			case *ssa.UnOp:
				refs := *instr.Referrers()
				if len(refs) == 0 {
					return true
				}
				for _, r := range refs {
					if v, ok := r.(ssa.Value); ok {
						if ptr, ok := v.Type().(*types.Pointer); !ok || !isNamedType(ptr.Elem(), "io", "ReadCloser") {
							continue
						}
						vrefs := *v.Referrers()
						for _, vref := range vrefs {
							if vref, ok := vref.(*ssa.UnOp); ok {
								vrefs := *vref.Referrers()
								if len(vrefs) == 0 {
									return true
								}
								for _, vref := range vrefs {
									if c, ok := vref.(*ssa.Call); ok {
										if c.Call.Method != nil && c.Call.Method.Name() == closeMethod {
											return !called
										}
									}
								}
							}
						}
					}

				}
			default:
				return r.isopen(b, i) || !called
			}
		}
	}
	return false
}

// isNamedType reports whether t is the named type path.name.
func isNamedType(t types.Type, path, name string) bool {
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := n.Obj()
	return obj.Name() == name && obj.Pkg() != nil && obj.Pkg().Path() == path
}
