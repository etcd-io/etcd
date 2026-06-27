package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mgechev/revive/internal/typeparams"
	"github.com/mgechev/revive/lint"
)

// disabledChecks store ignored warnings types.
type disabledChecks struct {
	Const            bool
	Function         bool
	Method           bool
	PrivateReceivers bool
	PublicInterfaces bool
	RepetitiveNames  bool
	Type             bool
	Var              bool
}

const (
	checkNamePrivateReceivers = "privateReceivers"
	checkNamePublicInterfaces = "publicInterfaces"
	checkNameStuttering       = "stuttering"
)

// isDisabled returns true if the given check is disabled, false otherwise.
func (dc *disabledChecks) isDisabled(checkName string) bool {
	switch checkName {
	case "var":
		return dc.Var
	case "const":
		return dc.Const
	case "function":
		return dc.Function
	case "method":
		return dc.Method
	case checkNamePrivateReceivers:
		return dc.PrivateReceivers
	case checkNamePublicInterfaces:
		return dc.PublicInterfaces
	case checkNameStuttering:
		return dc.RepetitiveNames
	case "type":
		return dc.Type
	default:
		return false
	}
}

var commonMethods = map[string]bool{
	"Error":     true,
	"Read":      true,
	"ServeHTTP": true,
	"String":    true,
	"Write":     true,
	"Unwrap":    true,
}

// ExportedRule lints naming and commenting conventions on exported symbols.
type ExportedRule struct {
	isRepetitiveMsg string
	disabledChecks  disabledChecks
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configure makes the rule implement the [lint.ConfigurableRule] interface.
func (r *ExportedRule) Configure(arguments lint.Arguments) error {
	r.disabledChecks = disabledChecks{PrivateReceivers: true, PublicInterfaces: true}
	r.isRepetitiveMsg = "stutters"
	for _, flag := range arguments {
		switch flag := flag.(type) {
		case string:
			switch {
			case isRuleOption(flag, "checkPrivateReceivers"):
				r.disabledChecks.PrivateReceivers = false
			case isRuleOption(flag, "disableStutteringCheck"):
				r.disabledChecks.RepetitiveNames = true
			case isRuleOption(flag, "sayRepetitiveInsteadOfStutters"):
				r.isRepetitiveMsg = "is repetitive"
			case isRuleOption(flag, "checkPublicInterface"):
				r.disabledChecks.PublicInterfaces = false
			case isRuleOption(flag, "disableChecksOnConstants"):
				r.disabledChecks.Const = true
			case isRuleOption(flag, "disableChecksOnFunctions"):
				r.disabledChecks.Function = true
			case isRuleOption(flag, "disableChecksOnMethods"):
				r.disabledChecks.Method = true
			case isRuleOption(flag, "disableChecksOnTypes"):
				r.disabledChecks.Type = true
			case isRuleOption(flag, "disableChecksOnVariables"):
				r.disabledChecks.Var = true
			default:
				return fmt.Errorf("unknown configuration flag %s for %s rule", flag, r.Name())
			}
		default:
			return fmt.Errorf("invalid argument for the %s rule: expecting a string, got %T", r.Name(), flag)
		}
	}

	return nil
}

// Apply applies the rule to given file.
func (r *ExportedRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if !file.IsImportable() {
		return nil
	}

	var failures []lint.Failure
	walker := lintExported{
		file: file,
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
		genDeclMissingComments: map[*ast.GenDecl]bool{},
		isRepetitiveMsg:        r.isRepetitiveMsg,
		disabledChecks:         r.disabledChecks,
	}

	ast.Walk(&walker, file.AST)

	return failures
}

// Name returns the rule name.
func (*ExportedRule) Name() string {
	return "exported"
}

type lintExported struct {
	file                   *lint.File
	lastGenDecl            *ast.GenDecl // the last visited general declaration in the AST
	genDeclMissingComments map[*ast.GenDecl]bool
	onFailure              func(lint.Failure)
	isRepetitiveMsg        string
	disabledChecks         disabledChecks
}

func (w *lintExported) lintFuncDoc(fn *ast.FuncDecl) {
	if !ast.IsExported(fn.Name.Name) {
		return // func is unexported, nothing to do
	}

	kind := "function"
	name := fn.Name.Name
	if isMethod := fn.Recv != nil && len(fn.Recv.List) > 0; isMethod {
		if !w.mustCheckMethod(fn) {
			return
		}

		kind = "method"
		recv := typeparams.ReceiverType(fn)
		name = recv + "." + name
	}

	if w.disabledChecks.isDisabled(kind) {
		return
	}

	status := w.checkGoDocStatus(fn.Doc, fn.Name.Name)
	switch status {
	case exportedGoDocStatusOK:
		return // comment is fine
	case exportedGoDocStatusMissing:
		w.addFailuref(fn, status.confidence(), lint.FailureCategoryComments,
			"exported %s %s should have comment or be unexported", kind, name,
		)
		return
	}

	firstCommentLine := w.firstCommentLine(fn.Doc)
	w.addFailuref(fn.Doc, status.confidence(), lint.FailureCategoryComments,
		`comment on exported %s %s should be of the form "%s ..."%s`, kind, name, fn.Name.Name, status.correctionHint(firstCommentLine),
	)
}

func (*lintExported) hasPrefixInsensitive(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix))
}

func (*lintExported) stripFirstRune(s string) string {
	// Decode the first rune to handle multi-byte characters.
	firstRune, size := utf8.DecodeRuneInString(s)
	if firstRune == utf8.RuneError {
		return s // no valid first rune found
	}

	// Return the string without the first rune.
	return s[size:]
}

func (w *lintExported) checkRepetitiveNames(id *ast.Ident, thing string) {
	if w.disabledChecks.RepetitiveNames {
		return
	}

	pkg, name := w.file.AST.Name.Name, id.Name
	if !ast.IsExported(name) {
		// unexported name
		return
	}
	// A name is repetitive if the package name is a strict prefix
	// and the next character of the name starts a new word.
	if len(name) <= len(pkg) {
		// name is too short to be a repetition.
		// This permits the name to be the same as the package name.
		return
	}
	if !strings.EqualFold(pkg, name[:len(pkg)]) {
		return
	}
	// We can assume the name is well-formed UTF-8.
	// If the next rune after the package name is uppercase or an underscore
	// the it's starting a new word and thus this name is repetitive.
	rem := name[len(pkg):]
	if next, _ := utf8.DecodeRuneInString(rem); next == '_' || unicode.IsUpper(next) {
		w.addFailuref(id, 0.8, lint.FailureCategoryNaming,
			"%s name will be used as %s.%s by other packages, and that %s; consider calling this %s", thing, pkg, name, w.isRepetitiveMsg, rem,
		)
	}
}

var articles = [...]string{"A", "An", "The", "This"}

func (w *lintExported) lintTypeDoc(t *ast.TypeSpec, doc *ast.CommentGroup, firstCommentLine string) {
	if w.disabledChecks.isDisabled("type") {
		return
	}

	typeName := t.Name.Name

	if !ast.IsExported(typeName) {
		return
	}

	if firstCommentLine == "" {
		w.addFailuref(t, 1, lint.FailureCategoryComments,
			"exported type %v should have comment or be unexported", t.Name,
		)
		return
	}

	expectedPrefix := typeName
	for _, a := range articles {
		if typeName == a {
			continue
		}
		var found bool
		if firstCommentLine, found = strings.CutPrefix(firstCommentLine, a+" "); found {
			expectedPrefix = a + " " + typeName
			break
		}
	}

	status := w.checkGoDocStatus(doc, expectedPrefix)
	if status == exportedGoDocStatusOK {
		return
	}
	w.addFailuref(doc, status.confidence(), lint.FailureCategoryComments,
		`comment on exported type %v should be of the form "%s ..." (with optional leading article)%s`, t.Name, typeName, status.correctionHint(firstCommentLine),
	)
}

// checkValueNames returns true if names check, false otherwise.
func (w *lintExported) checkValueNames(names []*ast.Ident, nodeToBlame ast.Node, kind string) bool {
	// Check that none are exported except for the first.
	if len(names) < 2 {
		return true // nothing to check
	}

	for _, n := range names[1:] {
		if ast.IsExported(n.Name) {
			w.addFailuref(nodeToBlame, 1, lint.FailureCategoryComments,
				"exported %s %s should have its own declaration", kind, n.Name,
			)
			return false
		}
	}

	return true
}

func (w *lintExported) lintValueSpecDoc(vs *ast.ValueSpec, gd *ast.GenDecl, genDeclMissingComments map[*ast.GenDecl]bool) {
	kind := "var"
	if gd.Tok == token.CONST {
		kind = "const"
	}

	if w.disabledChecks.isDisabled(kind) {
		return
	}

	if !w.checkValueNames(vs.Names, vs, kind) {
		return
	}

	// Only one name.
	name := vs.Names[0].Name
	if !ast.IsExported(name) {
		return
	}

	vsFirstCommentLine := w.firstCommentLine(vs.Doc)
	gdFirstCommentLine := w.firstCommentLine(gd.Doc)
	if vsFirstCommentLine == "" && gdFirstCommentLine == "" {
		if genDeclMissingComments[gd] {
			return
		}
		block := ""
		if kind == "const" && gd.Lparen.IsValid() {
			block = " (or a comment on this block)"
		}
		w.addFailuref(vs, 1, lint.FailureCategoryComments,
			"exported %s %s should have comment%s or be unexported", kind, name, block,
		)
		genDeclMissingComments[gd] = true
		return
	}

	// If this GenDecl has parens and a comment, we don't check its comment form.
	if gdFirstCommentLine != "" && gd.Lparen.IsValid() {
		return
	}

	// The relevant text to check will be on either vs.Doc or gd.Doc.
	// Use vs.Doc preferentially.
	var doc *ast.CommentGroup
	switch {
	case vsFirstCommentLine != "":
		doc = vs.Doc
	case vsFirstCommentLine != "" && gdFirstCommentLine == "":
		doc = vs.Comment
	default:
		doc = gd.Doc
	}
	firstCommentLine := w.firstCommentLine(doc)

	status := w.checkGoDocStatus(doc, name)
	if status == exportedGoDocStatusOK {
		return
	}
	w.addFailuref(doc, status.confidence(), lint.FailureCategoryComments,
		`comment on exported %s %s should be of the form "%s ..."%s`, kind, name, name, status.correctionHint(firstCommentLine),
	)
}

type exportedGoDocStatus int

const (
	exportedGoDocStatusOK exportedGoDocStatus = iota
	exportedGoDocStatusMissing
	exportedGoDocStatusCaseMismatch
	exportedGoDocStatusFirstLetterMismatch
	exportedGoDocStatusUnexpected
)

func (gds exportedGoDocStatus) confidence() float64 {
	if gds == exportedGoDocStatusUnexpected {
		return 0.8
	}
	return 1
}

func (gds exportedGoDocStatus) correctionHint(firstCommentLine string) string {
	firstWord := strings.Split(firstCommentLine, " ")[0]
	switch gds {
	case exportedGoDocStatusCaseMismatch:
		return ` by using its correct casing, not "` + firstWord + ` ..."`
	case exportedGoDocStatusFirstLetterMismatch:
		return ` to match its exported status, not "` + firstWord + ` ..."`
	}

	return ""
}

func (w *lintExported) checkGoDocStatus(comment *ast.CommentGroup, name string) exportedGoDocStatus {
	firstCommentLine := w.firstCommentLine(comment)
	if firstCommentLine == "" {
		return exportedGoDocStatusMissing
	}

	name = strings.TrimSpace(name)
	// Make sure the expected prefix has a space at the end.
	expectedPrefix := name + " "
	if strings.HasPrefix(firstCommentLine, expectedPrefix) {
		return exportedGoDocStatusOK
	}

	if !w.hasPrefixInsensitive(firstCommentLine, expectedPrefix) {
		return exportedGoDocStatusUnexpected
	}

	if strings.HasPrefix(w.stripFirstRune(firstCommentLine), w.stripFirstRune(expectedPrefix)) {
		// Only the first character differs, such as "sendJSON" became "SendJSON".
		// so we consider the scope has changed.
		return exportedGoDocStatusFirstLetterMismatch
	}

	return exportedGoDocStatusCaseMismatch
}

// firstCommentLine yields the first line of interest in comment group or "" if there is nothing of interest.
// An "interesting line" is a comment line that is neither a directive (e.g. `//go:...`) or a deprecation comment
// (lines from the first line with a prefix `// Deprecated:` to the end of the comment group).
// Empty or spaces-only lines are discarded.
func (*lintExported) firstCommentLine(comment *ast.CommentGroup) (result string) {
	if comment == nil {
		return ""
	}

	commentWithoutDirectives := comment.Text() // removes directives from the comment block
	for line := range strings.SplitSeq(commentWithoutDirectives, "\n") {
		line := strings.TrimSpace(line)
		if line == "" {
			continue // ignore empty lines
		}
		if strings.HasPrefix(line, "Deprecated: ") {
			break // ignore deprecation comment line and the subsequent lines of the original comment
		}

		result = line
		break // first non-directive/non-empty/non-deprecation comment line found
	}

	return result
}

func (w *lintExported) Visit(n ast.Node) ast.Visitor {
	switch v := n.(type) {
	case *ast.GenDecl:
		switch v.Tok {
		case token.IMPORT:
			return nil
		case token.CONST, token.TYPE, token.VAR:
			w.lastGenDecl = v
		}
		return w
	case *ast.FuncDecl:
		w.lintFuncDoc(v)
		if v.Recv == nil {
			// Only check for repetitive names on functions, not methods.
			// Method names are not used package-qualified.
			w.checkRepetitiveNames(v.Name, "func")
		}
		// Don't proceed inside funcs.
		return nil
	case *ast.TypeSpec:
		// inside a GenDecl, which usually has the doc
		doc := v.Doc

		fcl := w.firstCommentLine(doc)
		if fcl == "" {
			doc = w.lastGenDecl.Doc
			fcl = w.firstCommentLine(doc)
		}
		w.lintTypeDoc(v, doc, fcl)
		w.checkRepetitiveNames(v.Name, "type")

		if !w.disabledChecks.PublicInterfaces {
			if iface, ok := v.Type.(*ast.InterfaceType); ok {
				if ast.IsExported(v.Name.Name) {
					w.doCheckPublicInterface(v.Name.Name, iface)
				}
			}
		}

		return nil
	case *ast.ValueSpec:
		w.lintValueSpecDoc(v, w.lastGenDecl, w.genDeclMissingComments)
		return nil
	}
	return w
}

func (w *lintExported) doCheckPublicInterface(typeName string, iface *ast.InterfaceType) {
	for _, m := range iface.Methods.List {
		w.lintInterfaceMethod(typeName, m)
	}
}

func (w *lintExported) lintInterfaceMethod(typeName string, m *ast.Field) {
	if len(m.Names) == 0 {
		return
	}
	if !ast.IsExported(m.Names[0].Name) {
		return
	}

	name := m.Names[0].Name
	status := w.checkGoDocStatus(m.Doc, name)
	switch status {
	case exportedGoDocStatusOK:
		return // comment is fine
	case exportedGoDocStatusMissing:
		w.addFailuref(m, status.confidence(), lint.FailureCategoryComments,
			"public interface method %s.%s should be commented", typeName, name,
		)
		return
	}

	firstCommentLine := w.firstCommentLine(m.Doc)
	w.addFailuref(m.Doc, status.confidence(), lint.FailureCategoryComments,
		`comment on exported interface method %s.%s should be of the form "%s ..."%s`, typeName, name, name, status.correctionHint(firstCommentLine),
	)
}

// mustCheckMethod returns true if the method must be checked by this rule, false otherwise.
func (w *lintExported) mustCheckMethod(fn *ast.FuncDecl) bool {
	recv := typeparams.ReceiverType(fn)

	if !ast.IsExported(recv) && w.disabledChecks.PrivateReceivers {
		return false
	}

	name := fn.Name.Name
	if commonMethods[name] {
		return false
	}

	switch name {
	case "Len", "Less", "Swap":
		sortables := w.file.Pkg.Sortable()
		if sortables[recv] {
			return false
		}
	}

	return true
}

func (w *lintExported) addFailuref(node ast.Node, confidence float64, category lint.FailureCategory, message string, args ...any) {
	w.onFailure(lint.Failure{
		Node:       node,
		Confidence: confidence,
		Category:   category,
		Failure:    fmt.Sprintf(message, args...),
	})
}
