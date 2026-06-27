// Package taint provides a minimal taint analysis engine for gosec.
// It tracks data flow from sources (user input) to sinks (dangerous functions)
// using SSA form and call graph analysis.
//
// This implementation uses only golang.org/x/tools packages which gosec
// already depends on - no external dependencies required.
//
// Inspired by:
//   - github.com/google/capslock (call graph traversal pattern)
//   - gosec issue #1160 (requirements)
package taint

import (
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/ssa"
)

// maxTaintDepth limits recursion depth to prevent stack overflow on large codebases
const maxTaintDepth = 50

// maxCallerEdges caps the number of incoming call graph edges examined per function
// in isParameterTainted. CHA over-approximates call graphs (every interface method
// call fans out to ALL implementations), so a function can have thousands of callers.
// Real taint flows come from direct/nearby callers, not the 33rd+ CHA-generated edge.
const maxCallerEdges = 32

// isContextType checks if a type is context.Context.
// context.Context is a control-flow mechanism (deadlines, cancellation, request-scoped values)
// that does not carry user-controlled data relevant to taint sinks like XSS.
// Tainted context arguments (e.g., request.Context()) should not propagate taint
// to function return values, as the context doesn't flow as data to the output.
func isContextType(t types.Type) bool {
	// Unwrap pointer layers (e.g., *context.Context) to reach the named type.
	for {
		ptr, ok := t.(*types.Pointer)
		if !ok {
			break
		}
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == "context" && obj.Name() == "Context"
}

// Source defines where tainted data originates.
// Format: "package/path.TypeOrFunc" or "*package/path.Type" for pointer types.
type Source struct {
	// Package is the import path of the package containing the source (e.g., "net/http")
	Package string
	// Name is the type or function name that produces tainted data (e.g., "Request" for type, "Get" for function)
	Name string
	// Pointer indicates whether the source is a pointer type (true for *Type)
	Pointer bool
	// IsFunc marks this source as a function/method that returns tainted data
	// (e.g., os.Getenv, os.ReadFile). When false, Source is treated as a type
	// that is only tainted when received as a function parameter from external callers.
	IsFunc bool
}

// Sink defines a dangerous function that should not receive tainted data.
// Format: "(*package/path.Type).Method" or "package/path.Func"
type Sink struct {
	// Package is the import path of the package containing the sink (e.g., "database/sql")
	Package string
	// Receiver is the type name for methods (e.g., "DB"), or empty for package-level functions
	Receiver string
	// Method is the function or method name that represents the sink (e.g., "Query")
	Method string
	// Pointer indicates whether the receiver is a pointer type (true for *Type methods)
	Pointer bool
	// CheckArgs specifies which argument positions to check for taint (0-indexed).
	// For method calls, Args[0] is the receiver.
	// If nil or empty, all arguments are checked.
	// Examples:
	//   - SQL methods: [1] - only check query string (Args[1]), skip receiver
	//   - fmt.Fprintf: [1,2,3,...] - skip writer (Args[0]), check format and data
	CheckArgs []int

	// ArgTypeGuards constrains argument types before treating a call as a sink.
	// Key is the zero-based argument index; value is the required type expressed
	// as "import/path.TypeName" (e.g. "net/http.ResponseWriter").
	// The sink only fires when every guarded argument's type implements (or equals)
	// the named interface/type. When empty, no type constraint is applied.
	ArgTypeGuards map[int]string
}

// resolveOriginalType traces back through SSA interface-conversion instructions
// (ChangeInterface, MakeInterface) to recover the original value's type before
// any implicit widening to a broader interface (e.g. http.ResponseWriter → io.Writer).
func resolveOriginalType(v ssa.Value) types.Type {
	switch val := v.(type) {
	case *ssa.ChangeInterface:
		// ChangeInterface converts one interface type to another; trace through.
		return resolveOriginalType(val.X)
	case *ssa.MakeInterface:
		// MakeInterface boxes a concrete value into an interface; return the
		// concrete type (val.X.Type()), not the interface type.
		return val.X.Type()
	}
	return v.Type()
}

// guardsSatisfied returns true when every ArgTypeGuard declared in sink is
// satisfied by the concrete SSA argument types present in args.
//
// Interface guards are checked with types.Implements (handles pointer receivers
// and embedding). Concrete-type guards require exact types.Identical match.
// When sink.ArgTypeGuards is nil or empty the function always returns true.
//
// Argument types are resolved through ChangeInterface/MakeInterface so that
// an http.ResponseWriter passed where io.Writer is expected is still recognised
// as implementing http.ResponseWriter.
func guardsSatisfied(args []ssa.Value, sink Sink, prog *ssa.Program) bool {
	if len(sink.ArgTypeGuards) == 0 {
		return true
	}
	if prog == nil {
		return true // no program to resolve types against; skip guard
	}
	for argIdx, requiredTypePath := range sink.ArgTypeGuards {
		if argIdx >= len(args) {
			return false
		}
		// Resolve back through implicit interface conversions.
		argType := resolveOriginalType(args[argIdx])
		required := lookupNamedType(requiredTypePath, prog)
		if required == nil {
			// Type not found in the program — the guard cannot be satisfied.
			return false
		}
		iface, isIface := required.Underlying().(*types.Interface)
		if isIface {
			// Interface guard: accept if argType or *argType implements iface.
			if !types.Implements(argType, iface) &&
				!types.Implements(types.NewPointer(argType), iface) {
				return false
			}
		} else {
			// Concrete-type guard: require exact named-type identity.
			if !types.Identical(argType, required) &&
				!types.Identical(argType, types.NewPointer(required)) {
				return false
			}
		}
	}
	return true
}

// lookupNamedType resolves a fully-qualified type string of the form
// "import/path.TypeName" to a types.Type using the SSA program's package set.
// Returns nil when the package or type name is not found.
func lookupNamedType(typePath string, prog *ssa.Program) types.Type {
	lastDot := strings.LastIndex(typePath, ".")
	if lastDot < 0 {
		return nil
	}
	pkgPath := typePath[:lastDot]
	typeName := typePath[lastDot+1:]

	for _, pkg := range prog.AllPackages() {
		if pkg.Pkg == nil || pkg.Pkg.Path() != pkgPath {
			continue
		}
		member := pkg.Pkg.Scope().Lookup(typeName)
		if member == nil {
			continue
		}
		if tn, ok := member.(*types.TypeName); ok {
			return tn.Type()
		}
	}
	return nil
}

// Sanitizer defines a function that neutralizes taint.
// When tainted data passes through a sanitizer, it is no longer considered tainted.
type Sanitizer struct {
	// Package is the import path (e.g., "path/filepath")
	Package string
	// Receiver is the type name for methods, or empty for package-level functions
	Receiver string
	// Method is the function or method name (e.g., "Clean")
	Method string
	// Pointer indicates whether the receiver is a pointer type
	Pointer bool
}

// Result represents a detected taint flow from source to sink.
type Result struct {
	// Source is the origin of the tainted data
	Source Source
	// Sink is the dangerous function that receives the tainted data
	Sink Sink
	// SinkPos is the source code position of the sink call
	SinkPos token.Pos
	// Path is the sequence of functions from entry point to the sink
	Path []*ssa.Function
}

// Config holds taint analysis configuration.
type Config struct {
	// Sources is the list of data origins that produce tainted values
	Sources []Source
	// Sinks is the list of dangerous functions that should not receive tainted data
	Sinks []Sink
	// Sanitizers is the list of functions that neutralize taint (optional)
	Sanitizers []Sanitizer
}

// Analyzer performs taint analysis on SSA programs.
// paramKey identifies a specific parameter of a function for memoization.
type paramKey struct {
	fn       *ssa.Function
	paramIdx int
}

type Analyzer struct {
	config          *Config
	sources         map[string]Source   // keyed by full type string
	funcSrcs        map[string]Source   // function sources keyed by "pkg.Func"
	sinks           map[string]Sink     // keyed by full function string
	sanitizers      map[string]struct{} // keyed by full function string
	callGraph       *callgraph.Graph
	prog            *ssa.Program      // set at Analyze time for ArgTypeGuards resolution
	paramTaintCache map[paramKey]bool // caches true results from isParameterTainted
}

// SetCallGraph injects a precomputed call graph.
func (a *Analyzer) SetCallGraph(cg *callgraph.Graph) {
	a.callGraph = cg
}

// New creates a new taint analyzer with the given configuration.
func New(config *Config) *Analyzer {
	a := &Analyzer{
		config:     config,
		sources:    make(map[string]Source),
		funcSrcs:   make(map[string]Source),
		sinks:      make(map[string]Sink),
		sanitizers: make(map[string]struct{}),
	}

	// Index sources for fast lookup, separating type sources from function sources
	for _, src := range config.Sources {
		key := formatSourceKey(src)
		a.sources[key] = src
		if src.IsFunc {
			a.funcSrcs[key] = src
		}
	}

	// Index sinks for fast lookup
	for _, sink := range config.Sinks {
		key := formatSinkKey(sink)
		a.sinks[key] = sink
	}

	// Index sanitizers for fast lookup
	for _, san := range config.Sanitizers {
		key := formatSanitizerKey(san)
		a.sanitizers[key] = struct{}{}
	}

	return a
}

// formatSourceKey creates a lookup key for a source.
func formatSourceKey(src Source) string {
	key := src.Package + "." + src.Name
	if src.Pointer {
		key = "*" + key
	}
	return key
}

// formatSinkKey creates a lookup key for a sink.
func formatSinkKey(sink Sink) string {
	if sink.Receiver == "" {
		return sink.Package + "." + sink.Method
	}
	recv := sink.Package + "." + sink.Receiver
	if sink.Pointer {
		recv = "*" + recv
	}
	return "(" + recv + ")." + sink.Method
}

// formatSanitizerKey creates a lookup key for a sanitizer.
func formatSanitizerKey(san Sanitizer) string {
	if san.Receiver == "" {
		return san.Package + "." + san.Method
	}
	recv := san.Package + "." + san.Receiver
	if san.Pointer {
		recv = "*" + recv
	}
	return "(" + recv + ")." + san.Method
}

// Analyze performs taint analysis on the given SSA program.
// It returns all detected taint flows from sources to sinks.
func (a *Analyzer) Analyze(prog *ssa.Program, srcFuncs []*ssa.Function) []Result {
	if len(srcFuncs) == 0 {
		return nil
	}

	a.prog = prog

	if a.callGraph == nil {
		// Build call graph using Class Hierarchy Analysis (CHA).
		// CHA is fast and sound (no false negatives) but may have false positives.
		// For more precision, use VTA (Variable Type Analysis) instead.
		a.callGraph = cha.CallGraph(prog)
	}

	a.paramTaintCache = make(map[paramKey]bool)

	var results []Result

	// Find all sink calls in the program
	for _, fn := range srcFuncs {
		results = append(results, a.analyzeFunctionSinks(fn)...)
	}

	a.paramTaintCache = nil

	return results
}

// analyzeFunctionSinks finds sink calls in a function and traces taint.
func (a *Analyzer) analyzeFunctionSinks(fn *ssa.Function) []Result {
	if fn == nil || fn.Blocks == nil {
		return nil
	}

	var results []Result

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(*ssa.Call)
			if !ok {
				continue
			}

			// Check if this call is a sink
			sink, isSink := a.isSinkCall(call)
			if !isSink {
				continue
			}

			// Apply ArgTypeGuards: skip this sink if argument type constraints
			// are not satisfied (e.g. writer is not http.ResponseWriter).
			if !guardsSatisfied(call.Call.Args, sink, a.prog) {
				continue
			}

			// Determine which arguments to check for taint
			var argsToCheck []ssa.Value

			if len(sink.CheckArgs) > 0 {
				// Sink specifies which argument positions to check
				for _, idx := range sink.CheckArgs {
					if idx < len(call.Call.Args) {
						argsToCheck = append(argsToCheck, call.Call.Args[idx])
					}
				}
			} else {
				// No CheckArgs specified: check all arguments
				argsToCheck = call.Call.Args
			}

			// Check if any of the specified arguments are tainted
			for _, arg := range argsToCheck {
				if a.isTainted(arg, fn, make(map[ssa.Value]bool), 0) {
					results = append(results, Result{
						Sink:    sink,
						SinkPos: call.Pos(),
						Path:    a.buildPath(fn),
					})
					break
				}
			}
		}
	}

	return results
}

// isSinkCall checks if a call instruction is a sink and returns the sink info.
func (a *Analyzer) isSinkCall(call *ssa.Call) (Sink, bool) {
	// Try to get receiver info first (works for both concrete and interface calls)
	var pkg, receiverName, methodName string
	var isPointer bool

	// Check for method call (invoke or static with receiver)
	if call.Call.IsInvoke() {
		// Interface method call - receiver is in Call.Value, not Args
		if call.Call.Value != nil {
			recvType := call.Call.Value.Type()
			methodName = call.Call.Method.Name()

			// For interface calls, the type is usually a Named type pointing to the interface
			if named, ok := recvType.(*types.Named); ok {
				receiverName = named.Obj().Name()
				if pkgObj := named.Obj(); pkgObj != nil && pkgObj.Pkg() != nil {
					pkg = pkgObj.Pkg().Path()
				}
			}

			// Match against sinks (interface methods don't have Pointer field usually)
			for _, sink := range a.sinks {
				if sink.Package == pkg && sink.Receiver == receiverName && sink.Method == methodName {
					return sink, true
				}
			}
		}
	}

	// Try static callee (for non-interface method calls and functions)
	callee := call.Call.StaticCallee()
	if callee != nil {
		if callee.Pkg != nil && callee.Pkg.Pkg != nil {
			pkg = callee.Pkg.Pkg.Path()
		}
		methodName = callee.Name()

		// Check if it has a receiver (method call)
		if recv := callee.Signature.Recv(); recv != nil {
			recvType := recv.Type()
			if named, ok := recvType.(*types.Named); ok {
				receiverName = named.Obj().Name()
			}
			if ptr, ok := recvType.(*types.Pointer); ok {
				isPointer = true
				if named, ok := ptr.Elem().(*types.Named); ok {
					receiverName = named.Obj().Name()
				}
			}
		}
	}

	// Match against configured sinks
	for _, sink := range a.sinks {
		// Package must match
		if sink.Package != pkg {
			continue
		}

		// For method sinks (with receiver)
		if sink.Receiver != "" {
			if sink.Receiver == receiverName && sink.Method == methodName && sink.Pointer == isPointer {
				return sink, true
			}
		} else {
			// For function sinks (no receiver)
			if sink.Method == methodName && receiverName == "" {
				return sink, true
			}
		}
	}

	return Sink{}, false
}

// isSanitizerCall checks if a call instruction is a sanitizer.
func (a *Analyzer) isSanitizerCall(call *ssa.Call) bool {
	if len(a.sanitizers) == 0 {
		return false
	}

	callee := call.Call.StaticCallee()
	if callee == nil {
		return false
	}

	var pkg, receiverName, methodName string
	var isPointer bool

	if callee.Pkg != nil && callee.Pkg.Pkg != nil {
		pkg = callee.Pkg.Pkg.Path()
	}
	methodName = callee.Name()

	if recv := callee.Signature.Recv(); recv != nil {
		recvType := recv.Type()
		if named, ok := recvType.(*types.Named); ok {
			receiverName = named.Obj().Name()
		}
		if ptr, ok := recvType.(*types.Pointer); ok {
			isPointer = true
			if named, ok := ptr.Elem().(*types.Named); ok {
				receiverName = named.Obj().Name()
			}
		}
	}

	// Build key and check
	key := formatSanitizerKey(Sanitizer{
		Package:  pkg,
		Receiver: receiverName,
		Method:   methodName,
		Pointer:  isPointer,
	})
	_, found := a.sanitizers[key]
	return found
}

// isTainted recursively checks if a value is tainted (originates from a source).
//
// KEY DESIGN PRINCIPLE: Type-based source matching is ONLY applied to function
// parameters received from external callers and global variables. Locally
// constructed values of source types (e.g., http.NewRequest with a hardcoded
// URL) are NOT automatically considered tainted — their taintedness depends
// on whether the data flowing into them is tainted.
func (a *Analyzer) isTainted(v ssa.Value, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if v == nil {
		return false
	}

	// Prevent stack overflow on large codebases
	if depth > maxTaintDepth {
		return false
	}

	// Prevent infinite recursion
	if visited[v] {
		return false
	}
	visited[v] = true

	// Constants are compile-time literals and can never carry attacker-controlled
	// data. Short-circuit immediately — no taint possible.
	if _, ok := v.(*ssa.Const); ok {
		return false
	}

	// Trace back through SSA instructions
	switch val := v.(type) {
	case *ssa.Parameter:
		// Parameters are tainted if:
		// 1. Their type matches a source type AND they come from an external caller
		// 2. A caller passes tainted data to this parameter position
		return a.isParameterTainted(val, fn, visited, depth+1)

	case *ssa.Call:
		// FIRST: Check if this call is a sanitizer — sanitizers break the taint chain
		if a.isSanitizerCall(val) {
			return false
		}

		// Check if this is a known source function (e.g., os.Getenv, os.ReadFile)
		if a.isSourceFuncCall(val) {
			return true
		}

		// For method calls, check if the receiver carries taint.
		// This handles patterns like: req.URL.Query().Get("param")
		// where req is a tainted *http.Request parameter.
		if val.Call.IsInvoke() {
			// Interface method call — receiver is Call.Value
			if val.Call.Value != nil && a.isTainted(val.Call.Value, fn, visited, depth+1) {
				return true
			}
			// Also check non-receiver args for interface method calls.
			// Skip context.Context args — they don't carry user data to outputs.
			for _, arg := range val.Call.Args {
				if isContextType(arg.Type()) {
					continue
				}
				if a.isTainted(arg, fn, visited, depth+1) {
					return true
				}
			}
		} else if callee := val.Call.StaticCallee(); callee != nil && callee.Signature.Recv() != nil {
			// Static method call — receiver is Args[0]
			if len(val.Call.Args) > 0 && a.isTainted(val.Call.Args[0], fn, visited, depth+1) {
				return true
			}
			// Also check non-receiver arguments (Args[1:]) for methods.
			// For internal methods with bodies, use interprocedural analysis.
			// For external methods, conservatively propagate any tainted arg.
			if len(callee.Blocks) > 0 {
				if a.doTaintedArgsFlowToReturn(val, callee, fn, visited, depth+1) {
					return true
				}
			} else if len(val.Call.Args) > 1 {
				// Skip context.Context args — they don't carry user data to outputs.
				for _, arg := range val.Call.Args[1:] {
					if isContextType(arg.Type()) {
						continue
					}
					if a.isTainted(arg, fn, visited, depth+1) {
						return true
					}
				}
			}
		}

		// For non-method calls (plain functions), check if data-carrying arguments
		// are tainted AND actually flow to the return value.
		if callee := val.Call.StaticCallee(); callee != nil {
			if callee.Signature.Recv() == nil {
				if len(callee.Blocks) > 0 {
					// Internal function with available body — use interprocedural
					// analysis to check if tainted args actually influence the return.
					if a.doTaintedArgsFlowToReturn(val, callee, fn, visited, depth+1) {
						return true
					}
				} else {
					// External function (no body) — conservatively assume any
					// tainted arg taints the return. This is correct for stdlib
					// data-transformation functions (string ops, fmt, etc.).
					// Skip context.Context args — they don't carry user data to outputs.
					for _, arg := range val.Call.Args {
						if isContextType(arg.Type()) {
							continue
						}
						if a.isTainted(arg, fn, visited, depth+1) {
							return true
						}
					}
				}
			}
		}

		// Check for builtin calls (append, copy, string conversion, etc.)
		if _, ok := val.Call.Value.(*ssa.Builtin); ok {
			for _, arg := range val.Call.Args {
				if a.isTainted(arg, fn, visited, depth+1) {
					return true
				}
			}
		}

	case *ssa.FieldAddr:
		// Field access on a struct — use field-sensitive analysis.
		// Instead of blindly propagating taint from the parent struct, we
		// check whether this specific field carries tainted data.
		return a.isFieldAccessTainted(val, fn, visited, depth+1)

	case *ssa.IndexAddr:
		// Index into a tainted slice/array
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.UnOp:
		// Unary operation (like pointer dereference)
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.BinOp:
		// Binary operation - tainted if either operand is tainted
		return a.isTainted(val.X, fn, visited, depth+1) || a.isTainted(val.Y, fn, visited, depth+1)

	case *ssa.Phi:
		// Phi node - tainted if any edge is tainted
		for _, edge := range val.Edges {
			if a.isTainted(edge, fn, visited, depth+1) {
				return true
			}
		}

	case *ssa.Extract:
		// Extract from tuple - check the tuple
		return a.isTainted(val.Tuple, fn, visited, depth+1)

	case *ssa.TypeAssert:
		// Type assertion - check the underlying value
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.MakeInterface:
		// Interface creation - check the underlying value
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.Slice:
		// Slice operation - check the sliced value
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.Convert:
		// Type conversion - check the converted value
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.ChangeType:
		// Type change - check the underlying value
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.Alloc:
		// Allocation - check referrers for assignments
		for _, ref := range *val.Referrers() {
			// Direct stores to the allocation
			if store, ok := ref.(*ssa.Store); ok {
				if a.isTainted(store.Val, fn, visited, depth+1) {
					return true
				}
			}
			// For arrays/slices, check stores to indexed addresses (e.g., varargs)
			if indexAddr, ok := ref.(*ssa.IndexAddr); ok {
				if indexRefs := indexAddr.Referrers(); indexRefs != nil {
					for _, indexRef := range *indexRefs {
						if store, ok := indexRef.(*ssa.Store); ok {
							if a.isTainted(store.Val, fn, visited, depth+1) {
								return true
							}
						}
					}
				}
			}
		}

	case *ssa.Lookup:
		// Map/string lookup - check the map/string
		return a.isTainted(val.X, fn, visited, depth+1)

	case *ssa.MakeSlice:
		// MakeSlice - check if it's being populated with tainted data
		if refs := val.Referrers(); refs != nil {
			for _, ref := range *refs {
				if store, ok := ref.(*ssa.Store); ok {
					if a.isTainted(store.Val, fn, visited, depth+1) {
						return true
					}
				}
				if call, ok := ref.(*ssa.Call); ok {
					for _, arg := range call.Call.Args {
						if arg == val {
							continue // Skip the slice itself
						}
						if a.isTainted(arg, fn, visited, depth+1) {
							return true
						}
					}
				}
			}
		}
		return false

	case *ssa.MakeMap, *ssa.MakeChan:
		// New maps/channels are not tainted by default
		return false

	case *ssa.Const:
		// Constants are never tainted
		return false

	case *ssa.Global:
		// Global variables - check if configured as a known source (e.g., os.Args)
		if val.Pkg != nil && val.Pkg.Pkg != nil {
			globalKey := val.Pkg.Pkg.Path() + "." + val.Name()
			if _, ok := a.sources[globalKey]; ok {
				return true
			}
		}
		return false

	case *ssa.FreeVar:
		// Free variables in closures - trace to the enclosing scope's binding.
		// This handles closures like filepath.WalkDir callbacks where a variable
		// from the outer scope is captured.
		return a.isFreeVarTainted(val, fn, visited, depth+1)

	default:
		// Unhandled SSA instruction type - be conservative and don't propagate taint
		// to avoid false positives, but this might cause false negatives
		return false
	}

	return false
}

// isSourceType checks if a type matches any configured source type.
// This is used specifically for parameter checking, NOT for general value checking.
func (a *Analyzer) isSourceType(t types.Type) bool {
	if t == nil {
		return false
	}

	typeStr := t.String()

	// Direct match
	if _, ok := a.sources[typeStr]; ok {
		return true
	}

	// Check underlying type for named types
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil {
			key := obj.Pkg().Path() + "." + obj.Name()
			if _, ok := a.sources[key]; ok {
				return true
			}
			// Check pointer variant
			if _, ok := a.sources["*"+key]; ok {
				return true
			}
		}
	}

	// Check pointer types
	if ptr, ok := t.(*types.Pointer); ok {
		return a.isSourceType(ptr.Elem())
	}

	return false
}

// mayHaveExternalCallers reports whether fn could be invoked by code outside
// the analyzed package — code that is invisible to the call graph.
//
// Exported bare functions (non-methods) are the primary case: frameworks
// register them via dynamic dispatch that CHA cannot resolve, so the call
// graph may lack edges even though the function IS called at runtime.
//
// Methods with a receiver are excluded because CHA resolves interface dispatch
// to concrete methods, so their callers are generally visible in the graph.
// Unexported functions are only callable within the package, and the call graph
// covers intra-package calls comprehensively.
func mayHaveExternalCallers(fn *ssa.Function) bool {
	if fn.Signature == nil {
		return false
	}
	// Methods — CHA handles interface dispatch; callers are visible.
	if fn.Signature.Recv() != nil {
		return false
	}
	// Closures / anonymous functions are never exported.
	if fn.Parent() != nil {
		return false
	}
	// Exported bare function — may be called by external frameworks.
	return token.IsExported(fn.Name())
}

// isSourceFuncCall checks if a call invokes a known source function
// (a function explicitly configured as producing tainted data, e.g., os.Getenv).
func (a *Analyzer) isSourceFuncCall(call *ssa.Call) bool {
	callee := call.Call.StaticCallee()
	if callee == nil {
		return false
	}

	if callee.Pkg != nil && callee.Pkg.Pkg != nil {
		pkg := callee.Pkg.Pkg.Path()
		funcKey := pkg + "." + callee.Name()
		if src, ok := a.sources[funcKey]; ok && src.IsFunc {
			return true
		}
	}

	return false
}

// isParameterTainted checks if a function parameter receives tainted data.
//
// A parameter is tainted if:
// 1. Its type matches a configured source type (e.g., *http.Request in a handler)
// 2. Any caller passes tainted data to the corresponding argument position
func (a *Analyzer) isParameterTainted(param *ssa.Parameter, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	// Prevent stack overflow
	if depth > maxTaintDepth {
		return false
	}

	// Resolve paramIdx early so we can use it for cache lookups.
	paramIdx := -1
	for i, p := range fn.Params {
		if p == param {
			paramIdx = i
			break
		}
	}

	// Check memoization cache (only true results are cached).
	if paramIdx >= 0 && a.paramTaintCache != nil {
		key := paramKey{fn: fn, paramIdx: paramIdx}
		if a.paramTaintCache[key] {
			return true
		}
	}

	// Use call graph to find callers and check their arguments
	if a.callGraph == nil {
		// No call graph: fall back to type-based auto-taint for source-typed params
		// (conservative — may produce false positives, but we have no callee info).
		if a.isSourceType(param.Type()) {
			if paramIdx >= 0 && a.paramTaintCache != nil {
				a.paramTaintCache[paramKey{fn: fn, paramIdx: paramIdx}] = true
			}
			return true
		}
		return false
	}

	node := a.callGraph.Nodes[fn]

	// Check if parameter type is a configured source type.
	//
	// Strategy:
	//   1. No callers in call graph → definite entry point → auto-taint.
	//   2. Exported bare function → may have invisible external callers
	//      (framework dispatch) → auto-taint to avoid false negatives.
	//   3. Has callers, not exported bare func → fall through to caller check.
	//
	// Case 2 addresses a class of false negatives where an internal caller
	// with safe args suppresses taint for an exported entry point that is
	// also called externally by a framework (issue #1629 + Barry review).
	// Methods are excluded because CHA resolves interface dispatch, making
	// their callers visible in the call graph.
	if a.isSourceType(param.Type()) {
		isEntryPoint := (node == nil || len(node.In) == 0)
		if isEntryPoint || mayHaveExternalCallers(fn) {
			if paramIdx >= 0 && a.paramTaintCache != nil {
				a.paramTaintCache[paramKey{fn: fn, paramIdx: paramIdx}] = true
			}
			return true
		}
		// Has known callers and is not a handler — fall through to verify
		// taint via those callers.
	}

	if node == nil {
		return false
	}

	if paramIdx < 0 {
		return false
	}

	// Compute the adjusted index ONCE outside the loop.
	adjustedIdx := paramIdx
	if fn.Signature.Recv() != nil {
		// In SSA, method parameters include the receiver at index 0.
		// fn.Params already includes the receiver, so paramIdx is correct
		// relative to fn.Params. But call site Args also include the receiver
		// at index 0 for bound methods. So we don't need to adjust—the
		// indices are already aligned.
		// However, for interface method invocations (IsInvoke), the receiver
		// is in Call.Value, not Args. We handle that separately below.
		adjustedIdx = paramIdx
	}

	// Check each caller, capping at maxCallerEdges to avoid combinatorial
	// explosion from CHA over-approximation of interface method calls.
	edgesChecked := 0
	for _, inEdge := range node.In {
		if edgesChecked >= maxCallerEdges {
			break
		}

		site := inEdge.Site
		if site == nil {
			continue
		}

		callArgs := site.Common().Args

		if adjustedIdx < len(callArgs) {
			edgesChecked++
			if a.isTainted(callArgs[adjustedIdx], inEdge.Caller.Func, visited, depth+1) {
				if a.paramTaintCache != nil {
					a.paramTaintCache[paramKey{fn: fn, paramIdx: paramIdx}] = true
				}
				return true
			}
		}
	}

	return false
}

// isFreeVarTainted checks if a closure's free variable is tainted.
// Free variables are captured from the enclosing function's scope.
func (a *Analyzer) isFreeVarTainted(fv *ssa.FreeVar, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if depth > maxTaintDepth {
		return false
	}

	// Find the enclosing function that creates this closure
	parent := fn.Parent()
	if parent == nil {
		return false
	}

	// Find the MakeClosure instruction in the parent that creates fn
	for _, block := range parent.Blocks {
		for _, instr := range block.Instrs {
			mc, ok := instr.(*ssa.MakeClosure)
			if !ok {
				continue
			}
			// Check if this MakeClosure creates our function
			if mc.Fn != fn {
				continue
			}
			// mc.Bindings correspond to fn.FreeVars in the same order
			for i, binding := range mc.Bindings {
				if i < len(fn.FreeVars) && fn.FreeVars[i] == fv {
					return a.isTainted(binding, parent, visited, depth+1)
				}
			}
		}
	}

	return false
}

// isFieldAccessTainted checks whether a specific field of a struct carries tainted data.
//
// This is the core of field-sensitive taint tracking. Rather than treating
// the entire struct as tainted when any field is tainted, we trace the
// specific field to see if IT was assigned tainted data.
func (a *Analyzer) isFieldAccessTainted(fa *ssa.FieldAddr, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if depth > maxTaintDepth {
		return false
	}

	// CASE 1: The struct is a parameter of a known source type (e.g., *http.Request).
	// ALL fields of externally-supplied source types are considered tainted.
	if a.isSourceType(fa.X.Type()) {
		if _, ok := fa.X.(*ssa.Parameter); ok {
			return true
		}
		// If not a parameter but still a source type, trace the struct origin
		if a.isTainted(fa.X, fn, visited, depth) {
			return true
		}
		return false
	}

	// CASE 2: The struct was returned by a function call.
	// Use interprocedural analysis: look inside the callee to see if this
	// specific field index was assigned tainted data.
	if call, ok := fa.X.(*ssa.Call); ok {
		if callee := call.Call.StaticCallee(); callee != nil && callee.Blocks != nil {
			return a.isFieldTaintedViaCall(call, fa.Field, callee, fn, visited, depth)
		}
		// External function — fall back to checking if the call result is tainted
		return a.isTainted(fa.X, fn, visited, depth)
	}

	// CASE 3: The struct is from an Extract (multi-return call, e.g., job, err := NewJob(...)).
	if extract, ok := fa.X.(*ssa.Extract); ok {
		if call, ok := extract.Tuple.(*ssa.Call); ok {
			if callee := call.Call.StaticCallee(); callee != nil && callee.Blocks != nil {
				return a.isFieldTaintedViaCall(call, fa.Field, callee, fn, visited, depth)
			}
		}
		// Fall back
		return a.isTainted(fa.X, fn, visited, depth)
	}

	// CASE 4: The struct is a local Alloc. Check stores to this specific field.
	if alloc, ok := fa.X.(*ssa.Alloc); ok {
		return a.isFieldOfAllocTainted(alloc, fa.Field, fn, visited, depth)
	}

	// CASE 5: Pointer dereference (load) — trace through the pointer.
	if unop, ok := fa.X.(*ssa.UnOp); ok {
		return a.isFieldAccessOnPointerTainted(unop, fa.Field, fn, visited, depth)
	}

	// CASE 6: Phi node — field is tainted if tainted on any incoming edge.
	if phi, ok := fa.X.(*ssa.Phi); ok {
		for _, edge := range phi.Edges {
			if a.isFieldTaintedOnValue(edge, fa.Field, fn, visited, depth+1) {
				return true
			}
		}
		return false
	}

	// CASE 7: Nested field access — e.g., job.Rinse.Something
	if innerFA, ok := fa.X.(*ssa.FieldAddr); ok {
		return a.isFieldAccessTainted(innerFA, fn, visited, depth)
	}

	// Default: fall back to checking if the parent struct value is tainted.
	return a.isTainted(fa.X, fn, visited, depth)
}

// isFieldTaintedOnValue checks if a specific field of a value is tainted.
func (a *Analyzer) isFieldTaintedOnValue(v ssa.Value, fieldIdx int, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if v == nil || depth > maxTaintDepth {
		return false
	}

	switch val := v.(type) {
	case *ssa.Call:
		if callee := val.Call.StaticCallee(); callee != nil && callee.Blocks != nil {
			return a.isFieldTaintedViaCall(val, fieldIdx, callee, fn, visited, depth)
		}
		return a.isTainted(v, fn, visited, depth)
	case *ssa.Extract:
		if call, ok := val.Tuple.(*ssa.Call); ok {
			if callee := call.Call.StaticCallee(); callee != nil && callee.Blocks != nil {
				return a.isFieldTaintedViaCall(call, fieldIdx, callee, fn, visited, depth)
			}
		}
		return a.isTainted(v, fn, visited, depth)
	case *ssa.Alloc:
		return a.isFieldOfAllocTainted(val, fieldIdx, fn, visited, depth)
	case *ssa.Phi:
		if visited[v] {
			return false
		}
		visited[v] = true
		for _, edge := range val.Edges {
			if a.isFieldTaintedOnValue(edge, fieldIdx, fn, visited, depth+1) {
				return true
			}
		}
		return false
	default:
		return a.isTainted(v, fn, visited, depth)
	}
}

// isFieldOfAllocTainted checks if a specific field of a locally-allocated struct
// has been assigned tainted data.
func (a *Analyzer) isFieldOfAllocTainted(alloc *ssa.Alloc, fieldIdx int, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if alloc.Referrers() == nil {
		return false
	}
	for _, ref := range *alloc.Referrers() {
		fa, ok := ref.(*ssa.FieldAddr)
		if !ok || fa.Field != fieldIdx {
			continue
		}

		if fa.Referrers() == nil {
			continue
		}
		for _, faRef := range *fa.Referrers() {
			store, ok := faRef.(*ssa.Store)
			if !ok || store.Addr != fa {
				continue
			}
			if a.isTainted(store.Val, fn, visited, depth+1) {
				return true
			}
		}
	}
	return false
}

// isFieldAccessOnPointerTainted handles field access through a pointer dereference.
func (a *Analyzer) isFieldAccessOnPointerTainted(unop *ssa.UnOp, fieldIdx int, fn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	// Trace through the pointer to find the underlying value
	return a.isFieldTaintedOnValue(unop.X, fieldIdx, fn, visited, depth)
}

// isFieldTaintedViaCall performs interprocedural analysis to check if a specific
// field of the struct returned by a function call is tainted.
//
// It looks inside the callee to find the returned struct allocation and checks
// whether the specific field was assigned data derived from tainted arguments.
func (a *Analyzer) isFieldTaintedViaCall(call *ssa.Call, fieldIdx int, callee *ssa.Function, callerFn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if depth > maxTaintDepth || callee == nil {
		return false
	}

	// Prevent re-analyzing the same call site
	if visited[call] {
		return false
	}
	visited[call] = true

	// If we don't have SSA blocks (external function or no body), use fallback logic:
	// Assume the field is tainted if any argument to the constructor is tainted.
	if callee.Blocks == nil {
		for _, arg := range call.Call.Args {
			if a.isTainted(arg, callerFn, visited, depth) {
				return true
			}
		}
		return false
	}

	// Find all Return instructions in the callee
	for _, block := range callee.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			// Check each return value for our struct
			for _, retVal := range ret.Results {
				alloc := traceToAlloc(retVal)
				if alloc == nil {
					continue
				}
				// Check stores to this alloc's field at fieldIdx
				if a.isFieldOfAllocTaintedInCallee(alloc, fieldIdx, callee, call, callerFn, visited, depth+1) {
					return true
				}
			}
		}
	}

	return false
}

// isFieldOfAllocTaintedInCallee checks if a specific field of an allocated struct
// (inside a callee function) receives tainted data from the caller's arguments.
func (a *Analyzer) isFieldOfAllocTaintedInCallee(alloc *ssa.Alloc, fieldIdx int, callee *ssa.Function, call *ssa.Call, callerFn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if alloc.Referrers() == nil || depth > maxTaintDepth {
		return false
	}

	if visited[alloc] {
		return false
	}
	visited[alloc] = true
	for _, ref := range *alloc.Referrers() {
		fa, ok := ref.(*ssa.FieldAddr)
		if !ok || fa.Field != fieldIdx {
			continue
		}
		if fa.Referrers() == nil {
			continue
		}
		for _, faRef := range *fa.Referrers() {
			store, ok := faRef.(*ssa.Store)
			if !ok || store.Addr != fa {
				continue
			}
			// Check if the stored value traces back to a tainted caller argument.
			// Map callee parameters back to caller arguments.
			if a.isCalleValueTainted(store.Val, callee, call, callerFn, visited, depth+1) {
				return true
			}
		}
	}
	return false
}

// isCalleValueTainted checks if a value inside a callee is tainted, mapping
// callee parameters back to the actual caller arguments for interprocedural analysis.
func (a *Analyzer) isCalleValueTainted(v ssa.Value, callee *ssa.Function, call *ssa.Call, callerFn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if v == nil || depth > maxTaintDepth {
		return false
	}

	// Prevent infinite recursion on cyclic SSA value graphs
	if visited[v] {
		return false
	}
	visited[v] = true

	// If the value is a callee parameter, map it to the caller's argument
	if param, ok := v.(*ssa.Parameter); ok {
		for i, p := range callee.Params {
			if p == param && i < len(call.Call.Args) {
				return a.isTainted(call.Call.Args[i], callerFn, visited, depth)
			}
		}
		return false
	}

	// For constants, never tainted
	if _, ok := v.(*ssa.Const); ok {
		return false
	}

	// For calls within the callee, check if any tainted param flows in
	if innerCall, ok := v.(*ssa.Call); ok {
		// Check if it's a sanitizer
		if a.isSanitizerCall(innerCall) {
			return false
		}
		if a.isSourceFuncCall(innerCall) {
			return true
		}
		for _, arg := range innerCall.Call.Args {
			if a.isCalleValueTainted(arg, callee, call, callerFn, visited, depth+1) {
				return true
			}
		}
		return false
	}

	// For Extract (tuple unpacking), trace the tuple
	if extract, ok := v.(*ssa.Extract); ok {
		return a.isCalleValueTainted(extract.Tuple, callee, call, callerFn, visited, depth+1)
	}

	// For Phi, check all edges
	if phi, ok := v.(*ssa.Phi); ok {
		for _, edge := range phi.Edges {
			if a.isCalleValueTainted(edge, callee, call, callerFn, visited, depth+1) {
				return true
			}
		}
		return false
	}

	// For BinOp, check both sides
	if binop, ok := v.(*ssa.BinOp); ok {
		return a.isCalleValueTainted(binop.X, callee, call, callerFn, visited, depth+1) ||
			a.isCalleValueTainted(binop.Y, callee, call, callerFn, visited, depth+1)
	}

	// For Convert/ChangeType, trace through
	if conv, ok := v.(*ssa.Convert); ok {
		return a.isCalleValueTainted(conv.X, callee, call, callerFn, visited, depth+1)
	}
	if ct, ok := v.(*ssa.ChangeType); ok {
		return a.isCalleValueTainted(ct.X, callee, call, callerFn, visited, depth+1)
	}

	// For FieldAddr on a callee parameter (e.g., accessing a field of an arg struct)
	if fa, ok := v.(*ssa.FieldAddr); ok {
		return a.isCalleValueTainted(fa.X, callee, call, callerFn, visited, depth+1)
	}

	// For UnOp (pointer deref), trace through
	if unop, ok := v.(*ssa.UnOp); ok {
		return a.isCalleValueTainted(unop.X, callee, call, callerFn, visited, depth+1)
	}

	// For other SSA values, fall back to the callee-local taint check
	return a.isTainted(v, callee, visited, depth)
}

// doTaintedArgsFlowToReturn checks if any tainted argument to an internal function
// call actually influences the function's return value(s).
//
// This prevents false positives from constructor-like functions (e.g., NewJob)
// where only some arguments flow into the return struct, while others are stored
// in fields that don't affect the data being tracked.
func (a *Analyzer) doTaintedArgsFlowToReturn(call *ssa.Call, callee *ssa.Function, callerFn *ssa.Function, visited map[ssa.Value]bool, depth int) bool {
	if depth > maxTaintDepth {
		return false
	}

	// Identify which args are tainted.
	// Skip context.Context args — they don't carry user data to outputs.
	var taintedArgIndices []int
	for i, arg := range call.Call.Args {
		if isContextType(arg.Type()) {
			continue
		}
		if a.isTainted(arg, callerFn, visited, depth) {
			taintedArgIndices = append(taintedArgIndices, i)
		}
	}
	if len(taintedArgIndices) == 0 {
		return false
	}

	// Build a set of callee parameters that correspond to tainted caller args
	taintedParams := make(map[*ssa.Parameter]bool)
	for _, idx := range taintedArgIndices {
		if idx < len(callee.Params) {
			taintedParams[callee.Params[idx]] = true
		}
	}

	// Check if any tainted parameter flows to a Return instruction
	for _, block := range callee.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			for _, retVal := range ret.Results {
				if a.valueReachableFromParams(retVal, taintedParams, make(map[ssa.Value]bool), 0) {
					return true
				}
			}
		}
	}

	return false
}

// valueReachableFromParams checks if a value in a function is data-derived from
// any of the specified parameters. This is a lightweight reachability check
// within a single function body.
func (a *Analyzer) valueReachableFromParams(v ssa.Value, taintedParams map[*ssa.Parameter]bool, visited map[ssa.Value]bool, depth int) bool {
	if v == nil || depth > 30 || visited[v] {
		return false
	}
	visited[v] = true

	switch val := v.(type) {
	case *ssa.Parameter:
		return taintedParams[val]
	case *ssa.Const:
		return false
	case *ssa.Global:
		return false
	case *ssa.Alloc:
		// Check if any store to this alloc uses tainted data
		if val.Referrers() == nil {
			return false
		}
		for _, ref := range *val.Referrers() {
			if store, ok := ref.(*ssa.Store); ok && store.Addr == val {
				if a.valueReachableFromParams(store.Val, taintedParams, visited, depth+1) {
					return true
				}
			}
			// Also check FieldAddr stores (for struct allocs)
			if fa, ok := ref.(*ssa.FieldAddr); ok {
				if fa.Referrers() != nil {
					for _, faRef := range *fa.Referrers() {
						if store, ok := faRef.(*ssa.Store); ok && store.Addr == fa {
							if a.valueReachableFromParams(store.Val, taintedParams, visited, depth+1) {
								return true
							}
						}
					}
				}
			}
		}
		return false
	case *ssa.Call:
		// Check if any arg to this call comes from tainted params
		for _, arg := range val.Call.Args {
			if a.valueReachableFromParams(arg, taintedParams, visited, depth+1) {
				return true
			}
		}
		if val.Call.Value != nil {
			if a.valueReachableFromParams(val.Call.Value, taintedParams, visited, depth+1) {
				return true
			}
		}
		return false
	case *ssa.Phi:
		for _, edge := range val.Edges {
			if a.valueReachableFromParams(edge, taintedParams, visited, depth+1) {
				return true
			}
		}
		return false
	case *ssa.UnOp:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.BinOp:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1) ||
			a.valueReachableFromParams(val.Y, taintedParams, visited, depth+1)
	case *ssa.Convert:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.ChangeType:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.MakeInterface:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.TypeAssert:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.Slice:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.FieldAddr:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.IndexAddr:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	case *ssa.Extract:
		return a.valueReachableFromParams(val.Tuple, taintedParams, visited, depth+1)
	case *ssa.FreeVar:
		return false // Conservative: closures don't flow from params
	case *ssa.Lookup:
		return a.valueReachableFromParams(val.X, taintedParams, visited, depth+1)
	default:
		return false // Unknown SSA type — conservative, don't propagate
	}
}

// traceToAlloc follows a value back through SSA instructions to find
// the underlying Alloc instruction (struct allocation), if any.
func traceToAlloc(v ssa.Value) *ssa.Alloc {
	seen := make(map[ssa.Value]bool)
	return traceToAllocImpl(v, seen)
}

func traceToAllocImpl(v ssa.Value, seen map[ssa.Value]bool) *ssa.Alloc {
	if v == nil || seen[v] {
		return nil
	}
	seen[v] = true

	switch val := v.(type) {
	case *ssa.Alloc:
		return val
	case *ssa.Phi:
		for _, e := range val.Edges {
			if a := traceToAllocImpl(e, seen); a != nil {
				return a
			}
		}
		return nil
	case *ssa.MakeInterface:
		return traceToAllocImpl(val.X, seen)
	case *ssa.ChangeType:
		return traceToAllocImpl(val.X, seen)
	case *ssa.Convert:
		return traceToAllocImpl(val.X, seen)
	case *ssa.UnOp:
		return traceToAllocImpl(val.X, seen)
	default:
		return nil
	}
}

// buildPath constructs the call path from entry point to the sink.
func (a *Analyzer) buildPath(fn *ssa.Function) []*ssa.Function {
	if a.callGraph == nil {
		return []*ssa.Function{fn}
	}

	// BFS to find path from root to this function
	path := []*ssa.Function{fn}

	node := a.callGraph.Nodes[fn]
	if node == nil {
		return path
	}

	// Simple path: just trace callers up
	visited := make(map[*ssa.Function]bool)
	current := node

	for current != nil && len(current.In) > 0 {
		if visited[current.Func] {
			break
		}
		visited[current.Func] = true

		caller := current.In[0].Caller
		if caller == nil || caller.Func == nil {
			break
		}

		path = append([]*ssa.Function{caller.Func}, path...)
		current = caller
	}

	return path
}
