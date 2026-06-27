package rules

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type secretSerialization struct {
	issue.MetaData
	pattern *regexp.Regexp
	cache   sync.Map
}

type formatSpec struct {
	name            string
	tagKey          string
	marshalerMethod string // e.g. "MarshalJSON"; empty if no standard interface exists
	functionSinks   []functionSink
	methodSinks     []methodSink
}

type functionSink struct {
	pkgPath string
	names   []string
}

type methodSink struct {
	pkgPath  string
	typeName string
	method   string
}

type typeAnalysisCacheKey struct {
	typ    types.Type
	tagKey string
}

type sensitiveFieldMatch struct {
	fieldName     string
	serializedKey string
	found         bool
}

var g117Formats = []formatSpec{
	{
		name:            "JSON",
		tagKey:          "json",
		marshalerMethod: "MarshalJSON",
		functionSinks: []functionSink{
			{pkgPath: "encoding/json", names: []string{"Marshal", "MarshalIndent"}},
		},
		methodSinks: []methodSink{
			{pkgPath: "encoding/json", typeName: "Encoder", method: "Encode"},
		},
	},
	{
		name:            "YAML",
		tagKey:          "yaml",
		marshalerMethod: "MarshalYAML",
		functionSinks: []functionSink{
			{pkgPath: "go.yaml.in/yaml/v3", names: []string{"Marshal"}},
			{pkgPath: "gopkg.in/yaml.v3", names: []string{"Marshal"}},
			{pkgPath: "gopkg.in/yaml.v2", names: []string{"Marshal"}},
			{pkgPath: "sigs.k8s.io/yaml", names: []string{"Marshal"}},
		},
		methodSinks: []methodSink{
			{pkgPath: "go.yaml.in/yaml/v3", typeName: "Encoder", method: "Encode"},
			{pkgPath: "gopkg.in/yaml.v3", typeName: "Encoder", method: "Encode"},
			{pkgPath: "gopkg.in/yaml.v2", typeName: "Encoder", method: "Encode"},
		},
	},
	{
		name:            "XML",
		tagKey:          "xml",
		marshalerMethod: "MarshalXML",
		functionSinks: []functionSink{
			{pkgPath: "encoding/xml", names: []string{"Marshal", "MarshalIndent"}},
		},
		methodSinks: []methodSink{
			{pkgPath: "encoding/xml", typeName: "Encoder", method: "Encode"},
		},
	},
	{
		name:   "TOML",
		tagKey: "toml",
		functionSinks: []functionSink{
			{pkgPath: "github.com/pelletier/go-toml", names: []string{"Marshal"}},
			{pkgPath: "github.com/pelletier/go-toml/v2", names: []string{"Marshal"}},
		},
		methodSinks: []methodSink{
			{pkgPath: "github.com/pelletier/go-toml", typeName: "Encoder", method: "Encode"},
			{pkgPath: "github.com/pelletier/go-toml/v2", typeName: "Encoder", method: "Encode"},
			{pkgPath: "github.com/BurntSushi/toml", typeName: "Encoder", method: "Encode"},
		},
	},
}

func (r *secretSerialization) Match(n ast.Node, ctx *gosec.Context) (*issue.Issue, error) {
	callExpr, ok := n.(*ast.CallExpr)
	if !ok {
		return nil, nil
	}

	serializedArg, format, ok := r.findSerializedArgument(callExpr, ctx)
	if !ok || serializedArg == nil || ctx.Info == nil {
		return nil, nil
	}

	if isInsideCustomMarshaler(callExpr, ctx) {
		return nil, nil
	}

	typ := ctx.Info.TypeOf(serializedArg)
	if typ == nil {
		return nil, nil
	}

	if typeImplementsMarshaler(typ, format.marshalerMethod) {
		return nil, nil
	}

	match := r.findSensitiveFieldForType(typ, format.tagKey)
	if !match.found {
		return nil, nil
	}

	if compositeLitFieldIsTransformed(serializedArg, match.fieldName) {
		return nil, nil
	}

	msg := fmt.Sprintf("Marshaled struct field %q (%s key %q) matches secret pattern", match.fieldName, format.name, match.serializedKey)
	return ctx.NewIssue(callExpr, r.ID(), msg, r.Severity, r.Confidence), nil
}

// customMarshalerMethods lists method names that indicate a custom marshaler
// implementation. When a marshal call occurs inside one of these methods, the
// developer is explicitly controlling serialization, so G117 should not flag it.
var customMarshalerMethods = map[string]bool{
	"MarshalJSON": true,
	"MarshalYAML": true,
	"MarshalXML":  true,
	"MarshalText": true,
	"MarshalTOML": true,
	"MarshalBSON": true,
}

// isInsideCustomMarshaler reports whether callExpr is located inside a method
// whose name matches a known custom marshaler (e.g. MarshalJSON).
func isInsideCustomMarshaler(callExpr *ast.CallExpr, ctx *gosec.Context) bool {
	if ctx.Root == nil {
		return false
	}

	pos := callExpr.Pos()
	var found bool

	ast.Inspect(ctx.Root, func(n ast.Node) bool {
		if found {
			return false
		}
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			return true
		}
		// Check if the call is inside this function body.
		if pos < funcDecl.Body.Pos() || pos >= funcDecl.Body.End() {
			return true
		}
		// Must be a method (has a receiver) with a recognized marshaler name.
		if funcDecl.Recv != nil && funcDecl.Recv.NumFields() > 0 {
			if customMarshalerMethods[funcDecl.Name.Name] {
				found = true
			}
		}
		return false
	})

	return found
}

// typeImplementsMarshaler reports whether typ (or its element type for
// containers) has a method with the given name, indicating it implements a
// custom marshaler interface (e.g. json.Marshaler). When a type has a custom
// marshaler, the serialization library calls that method instead of serializing
// fields directly, making struct field analysis irrelevant.
func typeImplementsMarshaler(typ types.Type, methodName string) bool {
	if methodName == "" {
		return false
	}
	named := elementNamedType(typ)
	if named == nil {
		return false
	}
	// Check both value and pointer receiver methods via the pointer method set,
	// which is a superset of the value method set.
	mset := types.NewMethodSet(types.NewPointer(named))
	for i := 0; i < mset.Len(); i++ {
		if mset.At(i).Obj().Name() == methodName {
			return true
		}
	}
	return false
}

// elementNamedType unwraps pointers, slices, arrays, and maps to find the
// innermost Named type. Returns nil if no Named type is found.
func elementNamedType(typ types.Type) *types.Named {
	switch t := typ.(type) {
	case *types.Named:
		return t
	case *types.Pointer:
		return elementNamedType(t.Elem())
	case *types.Slice:
		return elementNamedType(t.Elem())
	case *types.Array:
		return elementNamedType(t.Elem())
	case *types.Map:
		return elementNamedType(t.Elem())
	}
	return nil
}

// compositeLitFieldIsTransformed checks whether expr is a composite literal
// in which the given field name is assigned a function call result. A function
// call indicates the value is being transformed (e.g. masked or redacted)
// before serialization.
func compositeLitFieldIsTransformed(expr ast.Expr, fieldName string) bool {
	// Unwrap address-of operator: &Struct{...}
	if unary, ok := expr.(*ast.UnaryExpr); ok {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != fieldName {
			continue
		}
		_, isCall := kv.Value.(*ast.CallExpr)
		return isCall
	}
	return false
}

func isNamedTypeInPackage(typ types.Type, pkgPath, typeName string) bool {
	if typ == nil {
		return false
	}

	switch t := typ.(type) {
	case *types.Pointer:
		return isNamedTypeInPackage(t.Elem(), pkgPath, typeName)
	case *types.Named:
		if obj := t.Obj(); obj != nil && obj.Name() == typeName {
			if pkg := obj.Pkg(); pkg != nil && pkg.Path() == pkgPath {
				return true
			}
		}
	}

	return false
}

func (r *secretSerialization) findSerializedArgument(callExpr *ast.CallExpr, ctx *gosec.Context) (ast.Expr, formatSpec, bool) {
	for _, format := range g117Formats {
		for _, sink := range format.functionSinks {
			if callMatchesPackageFunction(callExpr, ctx, sink.pkgPath, sink.names...) {
				if len(callExpr.Args) > 0 {
					return callExpr.Args[0], format, true
				}
				return nil, format, true
			}
		}

		for _, sink := range format.methodSinks {
			if !callMatchesMethodSink(callExpr, ctx, sink) {
				continue
			}
			if len(callExpr.Args) > 0 {
				return callExpr.Args[0], format, true
			}
			return nil, format, true
		}
	}

	return nil, formatSpec{}, false
}

func callMatchesMethodSink(callExpr *ast.CallExpr, ctx *gosec.Context, sink methodSink) bool {
	selector, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != sink.method {
		return false
	}

	if ctx != nil && ctx.Info != nil {
		receiverType := ctx.Info.TypeOf(selector.X)
		if isNamedTypeInPackage(receiverType, sink.pkgPath, sink.typeName) {
			return true
		}
	}

	constructorCall, ok := selector.X.(*ast.CallExpr)
	if !ok {
		return false
	}

	constructorName := "New" + sink.typeName
	if callMatchesPackageFunction(constructorCall, ctx, sink.pkgPath, constructorName) {
		return true
	}

	if strings.Contains(strings.ToLower(sink.pkgPath), "toml") {
		ctorSelector, ok := constructorCall.Fun.(*ast.SelectorExpr)
		if !ok || ctorSelector.Sel == nil || ctorSelector.Sel.Name != constructorName {
			return false
		}
		pkgIdent, ok := ctorSelector.X.(*ast.Ident)
		if !ok {
			return false
		}
		return importAliasPathContains(ctx, pkgIdent.Name, "toml")
	}

	return false
}

func callMatchesPackageFunction(callExpr *ast.CallExpr, ctx *gosec.Context, pkgPath string, names ...string) bool {
	if callExpr == nil || ctx == nil {
		return false
	}

	selector, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil {
		return false
	}

	matchedName := false
	for _, name := range names {
		if selector.Sel.Name == name {
			matchedName = true
			break
		}
	}
	if !matchedName {
		return false
	}

	if ctx.Info != nil {
		obj := ctx.Info.Uses[selector.Sel]
		if obj != nil && obj.Pkg() != nil && packagePathMatches(obj.Pkg().Path(), pkgPath) {
			return true
		}
	}

	if _, matched := gosec.MatchCallByPackage(callExpr, ctx, pkgPath, names...); matched {
		return true
	}

	pkgIdent, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	return importAliasMatchesPath(ctx, pkgIdent.Name, pkgPath)
}

func importAliasMatchesPath(ctx *gosec.Context, alias, pkgPath string) bool {
	if ctx == nil || ctx.Root == nil {
		return false
	}

	for _, imp := range ctx.Root.Imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !packagePathMatches(pathValue, pkgPath) {
			continue
		}

		importAlias := packageNameFromPath(pathValue)
		if imp.Name != nil {
			importAlias = imp.Name.Name
		}

		if importAlias == alias {
			return true
		}
	}

	return false
}

func importAliasPathContains(ctx *gosec.Context, alias, fragment string) bool {
	if ctx == nil || ctx.Root == nil {
		return false
	}

	for _, imp := range ctx.Root.Imports {
		pathValue, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		importAlias := packageNameFromPath(pathValue)
		if imp.Name != nil {
			importAlias = imp.Name.Name
		}

		if importAlias == alias && strings.Contains(strings.ToLower(pathValue), strings.ToLower(fragment)) {
			return true
		}
	}

	return false
}

func packageNameFromPath(path string) string {
	if idx := strings.LastIndexByte(path, '/'); idx >= 0 && idx+1 < len(path) {
		return path[idx+1:]
	}
	return path
}

func packagePathMatches(actual, expected string) bool {
	if actual == expected {
		return true
	}

	if strings.Contains(expected, "toml") {
		actualLower := strings.ToLower(actual)
		return strings.Contains(actualLower, "toml")
	}

	return false
}

func (r *secretSerialization) findSensitiveFieldForType(typ types.Type, tagKey string) sensitiveFieldMatch {
	return r.findSensitiveFieldForTypeWithVisited(typ, tagKey, make(map[types.Type]struct{}))
}

func (r *secretSerialization) findSensitiveFieldForTypeWithVisited(typ types.Type, tagKey string, visited map[types.Type]struct{}) sensitiveFieldMatch {
	if typ == nil {
		return sensitiveFieldMatch{}
	}

	cacheKey := typeAnalysisCacheKey{typ: typ, tagKey: tagKey}
	if cached, ok := r.cache.Load(cacheKey); ok {
		return cached.(sensitiveFieldMatch)
	}

	if _, seen := visited[typ]; seen {
		return sensitiveFieldMatch{}
	}
	visited[typ] = struct{}{}

	var match sensitiveFieldMatch

	switch t := typ.(type) {
	case *types.Named:
		match = r.findSensitiveFieldForTypeWithVisited(t.Underlying(), tagKey, visited)
	case *types.Pointer:
		match = r.findSensitiveFieldForTypeWithVisited(t.Elem(), tagKey, visited)
	case *types.Struct:
		match = r.findSensitiveSerializedField(t, tagKey)
	case *types.Slice:
		match = r.findSensitiveFieldForTypeWithVisited(t.Elem(), tagKey, visited)
	case *types.Array:
		match = r.findSensitiveFieldForTypeWithVisited(t.Elem(), tagKey, visited)
	case *types.Map:
		match = r.findSensitiveFieldForTypeWithVisited(t.Elem(), tagKey, visited)
	case *types.Interface:
		for i := 0; i < t.NumEmbeddeds(); i++ {
			match = r.findSensitiveFieldForTypeWithVisited(t.EmbeddedType(i), tagKey, visited)
			if match.found {
				break
			}
		}
	}

	r.cache.Store(cacheKey, match)
	return match
}

func (r *secretSerialization) findSensitiveSerializedField(st *types.Struct, tagKey string) sensitiveFieldMatch {
	if st == nil {
		return sensitiveFieldMatch{}
	}

	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		if field == nil || !field.Exported() || field.Name() == "_" {
			continue
		}

		if !isSecretCandidateType(field.Type()) {
			continue
		}

		effectiveKey, omitted := serializedNameFromTag(field.Name(), st.Tag(i), tagKey)
		if omitted {
			continue
		}

		if gosec.RegexMatchWithCache(r.pattern, field.Name()) || gosec.RegexMatchWithCache(r.pattern, effectiveKey) {
			return sensitiveFieldMatch{fieldName: field.Name(), serializedKey: effectiveKey, found: true}
		}
	}

	return sensitiveFieldMatch{}
}

func isSecretCandidateType(typ types.Type) bool {
	switch t := typ.(type) {
	case *types.Named:
		return isSecretCandidateType(t.Underlying())
	case *types.Basic:
		return t.Kind() == types.String
	case *types.Pointer:
		return isSecretCandidateType(t.Elem())
	case *types.Slice:
		if elemBasic, ok := t.Elem().(*types.Basic); ok && elemBasic.Kind() == types.Uint8 {
			return true
		}
		return isSecretCandidateType(t.Elem())
	case *types.Array:
		if elemBasic, ok := t.Elem().(*types.Basic); ok && elemBasic.Kind() == types.Uint8 {
			return true
		}
		return isSecretCandidateType(t.Elem())
	}

	return false
}

func serializedNameFromTag(defaultName, tag, tagKey string) (name string, omitted bool) {
	if tag == "" {
		return defaultName, false
	}

	tagValue := reflect.StructTag(tag).Get(tagKey)
	if tagValue == "" {
		return defaultName, false
	}
	if tagValue == "-" {
		return "", true
	}

	name = tagValue
	if idx := strings.IndexByte(tagValue, ','); idx >= 0 {
		name = tagValue[:idx]
	}

	if name == "" {
		return defaultName, false
	}

	return name, false
}

func NewSecretSerialization(id string, conf gosec.Config) (gosec.Rule, []ast.Node) {
	patternStr := `(?i)\b((?:api|access|auth|bearer|client|oauth|private|refresh|session|jwt)[_-]?(?:key|secret|token)s?|password|passwd|pwd|pass|secret|cred|jwt)\b`

	if val, ok := conf[id]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			if p, ok := m["pattern"].(string); ok && p != "" {
				patternStr = p
			}
		}
	}

	return &secretSerialization{
		pattern:  regexp.MustCompile(patternStr),
		MetaData: issue.NewMetaData(id, "Exported struct field appears to be a secret and is serialized by JSON/YAML/XML/TOML", issue.Medium, issue.Medium),
	}, []ast.Node{(*ast.CallExpr)(nil)}
}
