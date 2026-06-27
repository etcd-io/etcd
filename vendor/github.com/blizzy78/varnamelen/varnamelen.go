package varnamelen

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// varNameLen is an analyzer that checks that the length of a variable's name matches its usage scope.
// It will create a report for a variable's assignment if that variable has a short name, but its
// usage scope is not considered "small."
type varNameLen struct {
	// maxDistance is the longest distance, in source lines, that is being considered a "small scope."
	maxDistance int

	// minNameLength is the minimum length of a variable's name that is considered "long."
	minNameLength int

	// ignoreNames is an optional list of variable names that should be ignored completely.
	ignoreNames stringsValue

	// checkReceiver determines whether method receivers should be checked.
	checkReceiver bool

	// checkReturn determines whether named return values should be checked.
	checkReturn bool

	// ignoreTypeAssertOk determines whether "ok" variables that hold the bool return value of a type assertion should be ignored.
	ignoreTypeAssertOk bool

	// ignoreMapIndexOk determines whether "ok" variables that hold the bool return value of a map index should be ignored.
	ignoreMapIndexOk bool

	// ignoreChannelReceiveOk determines whether "ok" variables that hold the bool return value of a channel receive should be ignored.
	ignoreChannelReceiveOk bool

	// ignoreDeclarations is an optional list of variable declarations that should be ignored completely.
	ignoreDeclarations declarationsValue

	// checkTypeParameters determines whether type parameters should be checked.
	checkTypeParameters bool
}

// variable represents a declared variable.
type variable struct {
	// name is the name of the variable.
	name string

	// constant is true if the variable is actually a constant.
	constant bool

	// typ is the type of the variable.
	typ string

	// assign is the assign statement that declares the variable.
	assign *ast.AssignStmt

	// valueSpec is the value specification that declares the variable.
	valueSpec *ast.ValueSpec
}

// parameter represents a declared function or method parameter.
type parameter struct {
	// name is the name of the parameter.
	name string

	// typ is the type of the parameter.
	typ string

	// field is the declaration of the parameter.
	field *ast.Field
}

// typeParam represents a declared type parameter.
type typeParam struct {
	// name is the name of the type parameter.
	name string

	// typ is the type of the type parameter.
	typ string

	// field is the field that declares the type parameter.
	field *ast.Field
}

// declaration is a variable declaration.
type declaration struct {
	// name is the name of the variable.
	name string

	// constant is true if the variable is actually a constant.
	constant bool

	// typ is the type of the variable. Not used for constants.
	typ string
}

// importDeclaration is an import declaration.
type importDeclaration struct {
	// name is the short name or alias for the imported package. This is either the package's default name,
	// or the alias specified in the import statement.
	// Not used if self is true.
	name string

	// path is the full path to the imported package.
	path string

	// self is true when this is an implicit import declaration for the current package.
	self bool
}

const (
	// defaultMaxDistance is the default value for the maximum distance between the declaration of a variable and its usage
	// that is considered a "small scope."
	defaultMaxDistance = 5

	// defaultMinNameLength is the default value for the minimum length of a variable's name that is considered "long."
	defaultMinNameLength = 3
)

// conventionalDecls is a list of conventional variable declarations.
var conventionalDecls = []declaration{
	parseDeclaration("ctx context.Context"),

	parseDeclaration("b *testing.B"),
	parseDeclaration("f *testing.F"),
	parseDeclaration("m *testing.M"),
	parseDeclaration("pb *testing.PB"),
	parseDeclaration("t *testing.T"),
	parseDeclaration("tb testing.TB"),
}

// NewAnalyzer returns a new analyzer.
func NewAnalyzer() *analysis.Analyzer {
	vnl := varNameLen{
		maxDistance:        defaultMaxDistance,
		minNameLength:      defaultMinNameLength,
		ignoreNames:        stringsValue{},
		ignoreDeclarations: declarationsValue{},
	}

	analyzer := analysis.Analyzer{
		Name: "varnamelen",
		Doc: "checks that the length of a variable's name matches its scope\n\n" +
			"A variable with a short name can be hard to use if the variable is used\n" +
			"over a longer span of lines of code. A longer variable name may be easier\n" +
			"to comprehend.",

		Run: func(pass *analysis.Pass) (interface{}, error) {
			(&vnl).run(pass)
			return nil, nil
		},

		Requires: []*analysis.Analyzer{
			inspect.Analyzer,
		},
	}

	analyzer.Flags.IntVar(&vnl.maxDistance, "maxDistance", defaultMaxDistance, "maximum number of lines of variable usage scope considered 'short'")
	analyzer.Flags.IntVar(&vnl.minNameLength, "minNameLength", defaultMinNameLength, "minimum length of variable name considered 'long'")
	analyzer.Flags.Var(&vnl.ignoreNames, "ignoreNames", "comma-separated list of ignored variable names")
	analyzer.Flags.BoolVar(&vnl.checkReceiver, "checkReceiver", false, "check method receivers")
	analyzer.Flags.BoolVar(&vnl.checkReturn, "checkReturn", false, "check named return values")
	analyzer.Flags.BoolVar(&vnl.ignoreTypeAssertOk, "ignoreTypeAssertOk", false, "ignore 'ok' variables that hold the bool return value of a type assertion")
	analyzer.Flags.BoolVar(&vnl.ignoreMapIndexOk, "ignoreMapIndexOk", false, "ignore 'ok' variables that hold the bool return value of a map index")
	analyzer.Flags.BoolVar(&vnl.ignoreChannelReceiveOk, "ignoreChanRecvOk", false, "ignore 'ok' variables that hold the bool return value of a channel receive")
	analyzer.Flags.Var(&vnl.ignoreDeclarations, "ignoreDecls", "comma-separated list of ignored variable declarations")
	analyzer.Flags.BoolVar(&vnl.checkTypeParameters, "checkTypeParam", false, "check type parameters")

	return &analyzer
}

// Run applies v to a package, according to pass.
func (v *varNameLen) run(pass *analysis.Pass) {
	varToDist, paramToDist, returnToDist, typeParamToDist := v.distances(pass)

	v.checkVariables(pass, varToDist)
	v.checkParams(pass, paramToDist)
	v.checkReturns(pass, returnToDist)
	v.checkTypeParams(pass, typeParamToDist)
}

// checkVariables applies v to variables in varToDist.
func (v *varNameLen) checkVariables(pass *analysis.Pass, varToDist map[variable]int) { //nolint:gocognit // it's not that complicated
	for variable, dist := range varToDist {
		if v.ignoreNames.contains(variable.name) {
			continue
		}

		if v.ignoreDeclarations.matchVariable(variable) {
			continue
		}

		if v.checkNameAndDistance(variable.name, dist) {
			continue
		}

		if v.checkTypeAssertOk(variable) {
			continue
		}

		if v.checkMapIndexOk(variable) {
			continue
		}

		if v.checkChannelReceiveOk(variable) {
			continue
		}

		if variable.isConventional() {
			continue
		}

		if variable.assign != nil {
			pass.Reportf(variable.assign.Pos(), "%s name '%s' is too short for the scope of its usage", variable.kindName(), variable.name)
			continue
		}

		pass.Reportf(variable.valueSpec.Pos(), "%s name '%s' is too short for the scope of its usage", variable.kindName(), variable.name)
	}
}

// checkParams applies v to parameters in paramToDist.
func (v *varNameLen) checkParams(pass *analysis.Pass, paramToDist map[parameter]int) {
	for param, dist := range paramToDist {
		if v.ignoreNames.contains(param.name) {
			continue
		}

		if v.ignoreDeclarations.matchParameter(param) {
			continue
		}

		if v.checkNameAndDistance(param.name, dist) {
			continue
		}

		if param.isConventional() {
			continue
		}

		pass.Reportf(param.field.Pos(), "parameter name '%s' is too short for the scope of its usage", param.name)
	}
}

// checkReturns applies v to named return values in returnToDist.
func (v *varNameLen) checkReturns(pass *analysis.Pass, returnToDist map[parameter]int) {
	for returnValue, dist := range returnToDist {
		if v.ignoreNames.contains(returnValue.name) {
			continue
		}

		if v.ignoreDeclarations.matchParameter(returnValue) {
			continue
		}

		if v.checkNameAndDistance(returnValue.name, dist) {
			continue
		}

		pass.Reportf(returnValue.field.Pos(), "return value name '%s' is too short for the scope of its usage", returnValue.name)
	}
}

// checkTypeParams applies v to type parameters in paramToDist.
func (v *varNameLen) checkTypeParams(pass *analysis.Pass, paramToDist map[typeParam]int) {
	for param, dist := range paramToDist {
		if v.ignoreNames.contains(param.name) {
			continue
		}

		if v.ignoreDeclarations.matchTypeParameter(param) {
			continue
		}

		if v.checkNameAndDistance(param.name, dist) {
			continue
		}

		pass.Reportf(param.field.Pos(), "type parameter name '%s' is too short for the scope of its usage", param.name)
	}
}

// checkNameAndDistance returns true if name or dist are considered "short".
func (v *varNameLen) checkNameAndDistance(name string, dist int) bool {
	if len(name) >= v.minNameLength {
		return true
	}

	if dist <= v.maxDistance {
		return true
	}

	return false
}

// checkTypeAssertOk returns true if "ok" variables that hold the bool return value of a type assertion
// should be ignored, and if vari is such a variable.
func (v *varNameLen) checkTypeAssertOk(vari variable) bool {
	return v.ignoreTypeAssertOk && vari.isTypeAssertOk()
}

// checkMapIndexOk returns true if "ok" variables that hold the bool return value of a map index
// should be ignored, and if vari is such a variable.
func (v *varNameLen) checkMapIndexOk(vari variable) bool {
	return v.ignoreMapIndexOk && vari.isMapIndexOk()
}

// checkChannelReceiveOk returns true if "ok" variables that hold the bool return value of a channel receive
// should be ignored, and if vari is such a variable.
func (v *varNameLen) checkChannelReceiveOk(vari variable) bool {
	return v.ignoreChannelReceiveOk && vari.isChannelReceiveOk()
}

// distances returns maps of variables, parameters, return values, and type parameters mapping to their longest usage distances.
func (v *varNameLen) distances(pass *analysis.Pass) (map[variable]int, map[parameter]int, map[parameter]int, map[typeParam]int) {
	assignIdents, valueSpecIdents, paramIdents, returnIdents, typeParamIdents, imports, switches := v.identsAndImports(pass)

	varToDist := map[variable]int{}

	for _, ident := range assignIdents {
		assign := ident.Obj.Decl.(*ast.AssignStmt) //nolint:forcetypeassert // check is done in identsAndImports

		var typ string
		if isTypeSwitchAssign(assign, switches) {
			typ = "<type-switched>"
		} else {
			typ = shortTypeName(pass.TypesInfo.TypeOf(ident), imports)
		}

		variable := variable{
			name:   ident.Name,
			typ:    typ,
			assign: assign,
		}

		useLine := pass.Fset.Position(ident.NamePos).Line
		declLine := pass.Fset.Position(assign.Pos()).Line
		varToDist[variable] = useLine - declLine
	}

	for _, ident := range valueSpecIdents {
		valueSpec := ident.Obj.Decl.(*ast.ValueSpec) //nolint:forcetypeassert // check is done in identsAndImports

		variable := variable{
			name:      ident.Name,
			constant:  ident.Obj.Kind == ast.Con,
			typ:       shortTypeName(pass.TypesInfo.TypeOf(ident), imports),
			valueSpec: valueSpec,
		}

		useLine := pass.Fset.Position(ident.NamePos).Line
		declLine := pass.Fset.Position(valueSpec.Pos()).Line
		varToDist[variable] = useLine - declLine
	}

	paramToDist := map[parameter]int{}

	for _, ident := range paramIdents {
		field := ident.Obj.Decl.(*ast.Field) //nolint:forcetypeassert // check is done in identsAndImports

		param := parameter{
			name:  ident.Name,
			typ:   shortTypeName(pass.TypesInfo.TypeOf(field.Type), imports),
			field: field,
		}

		useLine := pass.Fset.Position(ident.NamePos).Line
		declLine := pass.Fset.Position(field.Pos()).Line
		paramToDist[param] = useLine - declLine
	}

	returnToDist := map[parameter]int{}

	for _, ident := range returnIdents {
		field := ident.Obj.Decl.(*ast.Field) //nolint:forcetypeassert // check is done in identsAndImports

		param := parameter{
			name:  ident.Name,
			typ:   shortTypeName(pass.TypesInfo.TypeOf(ident), imports),
			field: field,
		}

		useLine := pass.Fset.Position(ident.NamePos).Line
		declLine := pass.Fset.Position(field.Pos()).Line
		returnToDist[param] = useLine - declLine
	}

	typeParamToDist := map[typeParam]int{}

	for _, ident := range typeParamIdents {
		field := ident.Obj.Decl.(*ast.Field) //nolint:forcetypeassert // check is done in identsAndImports

		param := typeParam{
			name:  ident.Name,
			typ:   shortTypeName(pass.TypesInfo.TypeOf(field.Type), imports),
			field: field,
		}

		useLine := pass.Fset.Position(ident.NamePos).Line
		declLine := pass.Fset.Position(field.Pos()).Line
		typeParamToDist[param] = useLine - declLine
	}

	return varToDist, paramToDist, returnToDist, typeParamToDist
}

// identsAndImports returns Idents referencing assign statements, value specifications, parameters,
// return values, and type parameters, respectively, as well as import declarations, and type switch statements.
func (v *varNameLen) identsAndImports(pass *analysis.Pass) ([]*ast.Ident, []*ast.Ident, []*ast.Ident, []*ast.Ident, //nolint:gocognit,cyclop // this is complex stuff
	[]*ast.Ident, []importDeclaration, []*ast.TypeSwitchStmt) {
	inspector := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:forcetypeassert // inspect.Analyzer always returns *inspector.Inspector

	filter := []ast.Node{
		(*ast.ImportSpec)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
		(*ast.CompositeLit)(nil),
		(*ast.TypeSwitchStmt)(nil),
		(*ast.Ident)(nil),
	}

	assignIdents := []*ast.Ident{}
	valueSpecIdents := []*ast.Ident{}
	paramIdents := []*ast.Ident{}
	returnIdents := []*ast.Ident{}
	typeParamIdents := []*ast.Ident{}
	imports := []importDeclaration{}
	switches := []*ast.TypeSwitchStmt{}

	funcs := []*ast.FuncDecl{}
	methods := []*ast.FuncDecl{}
	funcLits := []*ast.FuncLit{}
	compositeLits := []*ast.CompositeLit{}

	inspector.Preorder(filter, func(node ast.Node) {
		switch node2 := node.(type) {
		case *ast.ImportSpec:
			decl, ok := importSpecToDecl(node2, pass.Pkg.Imports())
			if !ok {
				return
			}

			imports = append(imports, decl)

		case *ast.FuncDecl:
			funcs = append(funcs, node2)

			if node2.Recv == nil {
				return
			}

			methods = append(methods, node2)

		case *ast.FuncLit:
			funcLits = append(funcLits, node2)

		case *ast.CompositeLit:
			compositeLits = append(compositeLits, node2)

		case *ast.TypeSwitchStmt:
			switches = append(switches, node2)

		case *ast.Ident:
			if node2.Obj == nil {
				return
			}

			if isCompositeLitKey(node2, compositeLits) {
				return
			}

			switch objDecl := node2.Obj.Decl.(type) {
			case *ast.AssignStmt:
				assignIdents = append(assignIdents, node2)

			case *ast.ValueSpec:
				valueSpecIdents = append(valueSpecIdents, node2)

			case *ast.Field:
				switch {
				case isReceiver(objDecl, methods):
					if !v.checkReceiver {
						return
					}

					paramIdents = append(paramIdents, node2)

				case isReturn(objDecl, funcs, funcLits):
					if !v.checkReturn {
						return
					}

					returnIdents = append(returnIdents, node2)

				case isTypeParam(objDecl, funcs, funcLits):
					if !v.checkTypeParameters {
						return
					}

					typeParamIdents = append(typeParamIdents, node2)

				case isParam(objDecl, funcs, funcLits, methods):
					paramIdents = append(paramIdents, node2)
				}
			}
		}
	})

	imports = append(imports, importDeclaration{
		path: pass.Pkg.Path(),
		self: true,
	})

	sort.Slice(imports, func(a, b int) bool {
		// reversed: longest path first
		return len(imports[a].path) > len(imports[b].path)
	})

	return assignIdents, valueSpecIdents, paramIdents, returnIdents, typeParamIdents, imports, switches
}

func importSpecToDecl(spec *ast.ImportSpec, imports []*types.Package) (importDeclaration, bool) {
	path := strings.TrimSuffix(strings.TrimPrefix(spec.Path.Value, "\""), "\"")

	if spec.Name != nil {
		return importDeclaration{
			name: spec.Name.Name,
			path: path,
		}, true
	}

	for _, imp := range imports {
		if imp.Path() == path {
			return importDeclaration{
				name: imp.Name(),
				path: path,
			}, true
		}
	}

	return importDeclaration{}, false
}

// isTypeAssertOk returns true if v is an "ok" variable that holds the bool return value of a type assertion.
func (v variable) isTypeAssertOk() bool {
	if v.name != "ok" {
		return false
	}

	if v.assign == nil {
		return false
	}

	if len(v.assign.Lhs) != 2 {
		return false
	}

	ident, ok := v.assign.Lhs[1].(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name != "ok" {
		return false
	}

	if len(v.assign.Rhs) != 1 {
		return false
	}

	if _, ok := v.assign.Rhs[0].(*ast.TypeAssertExpr); !ok {
		return false
	}

	return true
}

// isMapIndexOk returns true if v is an "ok" variable that holds the bool return value of a map index.
func (v variable) isMapIndexOk() bool {
	if v.name != "ok" {
		return false
	}

	if v.assign == nil {
		return false
	}

	if len(v.assign.Lhs) != 2 {
		return false
	}

	ident, ok := v.assign.Lhs[1].(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name != "ok" {
		return false
	}

	if len(v.assign.Rhs) != 1 {
		return false
	}

	if _, ok := v.assign.Rhs[0].(*ast.IndexExpr); !ok {
		return false
	}

	return true
}

// isChannelReceiveOk returns true if v is an "ok" variable that holds the bool return value of a channel receive.
func (v variable) isChannelReceiveOk() bool {
	if v.name != "ok" {
		return false
	}

	if v.assign == nil {
		return false
	}

	if len(v.assign.Lhs) != 2 {
		return false
	}

	ident, ok := v.assign.Lhs[1].(*ast.Ident)
	if !ok {
		return false
	}

	if ident.Name != "ok" {
		return false
	}

	if len(v.assign.Rhs) != 1 {
		return false
	}

	unary, ok := v.assign.Rhs[0].(*ast.UnaryExpr)
	if !ok {
		return false
	}

	if unary.Op != token.ARROW {
		return false
	}

	return true
}

// isConventional returns true if v matches a conventional Go variable/parameter name and type,
// such as "ctx context.Context" or "t *testing.T".
func (v variable) isConventional() bool {
	for _, decl := range conventionalDecls {
		if v.match(decl) {
			return true
		}
	}

	return false
}

// match returns true if v matches decl.
func (v variable) match(decl declaration) bool {
	if v.name != decl.name {
		return false
	}

	if v.constant != decl.constant {
		return false
	}

	if v.constant {
		return true
	}

	if v.typ == "" {
		return false
	}

	return decl.matchType(v.typ)
}

// kindName returns "constant" if v.constant==true, else "variable".
func (v variable) kindName() string {
	if v.constant {
		return "constant"
	}

	return "variable"
}

// isReceiver returns true if field is a receiver parameter of any of the given methods.
func isReceiver(field *ast.Field, methods []*ast.FuncDecl) bool {
	for _, m := range methods {
		for _, recv := range m.Recv.List {
			if recv == field {
				return true
			}
		}
	}

	return false
}

// isReturn returns true if field is a return value of any of the given funcs.
func isReturn(field *ast.Field, funcs []*ast.FuncDecl, funcLits []*ast.FuncLit) bool { //nolint:gocognit // it's not that complicated
	for _, f := range funcs {
		if f.Type.Results == nil {
			continue
		}

		for _, r := range f.Type.Results.List {
			if r == field {
				return true
			}
		}
	}

	for _, f := range funcLits {
		if f.Type.Results == nil {
			continue
		}

		for _, r := range f.Type.Results.List {
			if r == field {
				return true
			}
		}
	}

	return false
}

// isParam returns true if field is a parameter of any of the given funcs.
func isParam(field *ast.Field, funcs []*ast.FuncDecl, funcLits []*ast.FuncLit, methods []*ast.FuncDecl) bool { //nolint:gocognit,cyclop // it's not that complicated
	for _, f := range funcs {
		if f.Type.Params == nil {
			continue
		}

		for _, p := range f.Type.Params.List {
			if p == field {
				return true
			}
		}
	}

	for _, f := range funcLits {
		if f.Type.Params == nil {
			continue
		}

		for _, p := range f.Type.Params.List {
			if p == field {
				return true
			}
		}
	}

	for _, m := range methods {
		if m.Type.Params == nil {
			continue
		}

		for _, p := range m.Type.Params.List {
			if p == field {
				return true
			}
		}
	}

	return false
}

// isCompositeLitKey returns true if ident is a key of any of the given composite literals.
func isCompositeLitKey(ident *ast.Ident, compositeLits []*ast.CompositeLit) bool {
	for _, cl := range compositeLits {
		if _, ok := cl.Type.(*ast.MapType); ok {
			continue
		}

		for _, kvExpr := range cl.Elts {
			kv, ok := kvExpr.(*ast.KeyValueExpr)
			if !ok {
				continue
			}

			if kv.Key == ident {
				return true
			}
		}
	}

	return false
}

// isTypeSwitchAssign returns true if assign is an assign statement of any of the given type switch statements.
func isTypeSwitchAssign(assign *ast.AssignStmt, switches []*ast.TypeSwitchStmt) bool {
	for _, s := range switches {
		if s.Assign == assign {
			return true
		}
	}

	return false
}

// isConventional returns true if v matches a conventional Go variable/parameter name and type,
// such as "ctx context.Context" or "t *testing.T".
func (p parameter) isConventional() bool {
	for _, decl := range conventionalDecls {
		if p.match(decl) {
			return true
		}
	}

	return false
}

// match returns whether p matches decl.
func (p parameter) match(decl declaration) bool {
	if p.name != decl.name {
		return false
	}

	return decl.matchType(p.typ)
}

// match returns whether p matches decl.
func (p typeParam) match(decl declaration) bool {
	if p.name != decl.name {
		return false
	}

	return decl.matchType(p.typ)
}

// parseDeclaration parses and returns a variable declaration parsed from decl.
func parseDeclaration(decl string) declaration {
	if strings.HasPrefix(decl, "const ") {
		return declaration{
			name:     strings.TrimPrefix(decl, "const "),
			constant: true,
		}
	}

	parts := strings.SplitN(decl, " ", 2)

	return declaration{
		name: parts[0],
		typ:  parts[1],
	}
}

// matchType returns true if typ matches d.typ.
func (d declaration) matchType(typ string) bool {
	return d.typ == typ
}

// shortTypeName returns the short name of typ, with respect to imports.
// For example, if package github.com/matryer/is is imported with alias "x",
// and typ represents []*github.com/matryer/is.I, shortTypeName will return "[]*x.I".
// For imports without aliases, the package's default name will be used.
func shortTypeName(typ types.Type, imports []importDeclaration) string {
	if typ == nil {
		return ""
	}

	typStr := typ.String()

	for _, imp := range imports {
		prefix := imp.path + "."

		replace := ""
		if !imp.self {
			replace = imp.name + "."
		}

		typStr = strings.ReplaceAll(typStr, prefix, replace)
	}

	return typStr
}
