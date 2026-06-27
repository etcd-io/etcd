package rule

import (
	"fmt"
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"github.com/mgechev/revive/lint"
)

// StringFormatRule lints strings and/or comments according to a set of regular expressions given as arguments.
type StringFormatRule struct {
	rules []stringFormatSubrule
}

// Apply applies the rule to the given file.
func (r *StringFormatRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	for i := range r.rules {
		r.rules[i].onFailure = onFailure
	}

	w := &lintStringFormatRule{
		rules: r.rules,
	}

	ast.Walk(w, file.AST)

	return failures
}

// Name returns the rule name.
func (*StringFormatRule) Name() string {
	return "string-format"
}

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *StringFormatRule) Configure(arguments lint.Arguments) error {
	for i, argument := range arguments {
		scopes, regex, negated, errorMessage, err := r.parseArgument(argument, i)
		if err != nil {
			return err
		}
		r.rules = append(r.rules, stringFormatSubrule{
			scopes:       scopes,
			regexp:       regex,
			negated:      negated,
			errorMessage: errorMessage,
		})
	}
	return nil
}

type lintStringFormatRule struct {
	rules []stringFormatSubrule
}

type stringFormatSubrule struct {
	onFailure    func(lint.Failure)
	scopes       stringFormatSubruleScopes
	regexp       *regexp.Regexp
	negated      bool
	errorMessage string
}

type stringFormatSubruleScopes []*stringFormatSubruleScope

type stringFormatSubruleScope struct {
	funcName string // Function name the rule is scoped to
	argument int    // (optional) Which argument in calls to the function is checked against the rule (the first argument is checked by default)
	field    string // (optional) If the argument to be checked is a struct, which member of the struct is checked against the rule (top level members only)
}

// identRegex matches valid function/struct field identifiers.
const identRegex = "[_A-Za-z][_A-Za-z0-9]*"

var parseStringFormatScope = regexp.MustCompile(
	fmt.Sprintf("^(%s(?:\\.%s)?)(?:\\[([0-9]+)\\](?:\\.(%s))?)?$", identRegex, identRegex, identRegex))

//revive:disable-next-line:function-result-limit
func (r *StringFormatRule) parseArgument(argument any, ruleNum int) (scopes stringFormatSubruleScopes, regex *regexp.Regexp, negated bool, errorMessage string, err error) {
	g, ok := argument.([]any) // Cast to generic slice first
	if !ok {
		return stringFormatSubruleScopes{}, regex, false, "", r.configError("argument is not a slice", ruleNum, 0)
	}
	if len(g) < 2 {
		return stringFormatSubruleScopes{}, regex, false, "", r.configError("less than two slices found in argument, scope and regex are required", ruleNum, len(g)-1)
	}
	rule := make([]string, len(g))
	for i, obj := range g {
		val, ok := obj.(string)
		if !ok {
			return stringFormatSubruleScopes{}, regex, false, "", r.configError("unexpected value, string was expected", ruleNum, i)
		}
		rule[i] = val
	}

	// Validate scope and regex length
	if rule[0] == "" {
		return stringFormatSubruleScopes{}, regex, false, "", r.configError("empty scope provided", ruleNum, 0)
	} else if len(rule[1]) < 2 {
		return stringFormatSubruleScopes{}, regex, false, "", r.configError("regex is too small (regexes should begin and end with '/')", ruleNum, 1)
	}

	// Parse rule scopes
	rawScopes := strings.Split(rule[0], ",")

	scopes = make([]*stringFormatSubruleScope, 0, len(rawScopes))
	for scopeNum, rawScope := range rawScopes {
		rawScope = strings.TrimSpace(rawScope)

		if rawScope == "" {
			return stringFormatSubruleScopes{}, regex, false, "", r.parseScopeError("empty scope in rule scopes:", ruleNum, 0, scopeNum)
		}

		scope := stringFormatSubruleScope{}
		matches := parseStringFormatScope.FindStringSubmatch(rawScope)
		if matches == nil {
			// The rule's scope didn't match the parsing regex at all, probably a configuration error
			return stringFormatSubruleScopes{}, regex, false, "", r.parseScopeError("unable to parse rule scope", ruleNum, 0, scopeNum)
		} else if len(matches) != 4 {
			// The rule's scope matched the parsing regex, but an unexpected number of submatches was returned, probably a bug
			return stringFormatSubruleScopes{}, regex, false, "",
				r.parseScopeError(fmt.Sprintf("unexpected number of submatches when parsing scope: %d, expected 4", len(matches)), ruleNum, 0, scopeNum)
		}
		scope.funcName = matches[1]
		if matches[2] != "" {
			var err error
			scope.argument, err = strconv.Atoi(matches[2])
			if err != nil {
				return stringFormatSubruleScopes{}, regex, false, "", r.parseScopeError("unable to parse argument number in rule scope", ruleNum, 0, scopeNum)
			}
		}
		if matches[3] != "" {
			scope.field = matches[3]
		}

		scopes = append(scopes, &scope)
	}

	// Strip / characters from the beginning and end of rule[1] before compiling
	negated = rule[1][0] == '!'
	offset := 1
	if negated {
		offset++
	}
	regex, errr := regexp.Compile(rule[1][offset : len(rule[1])-1])
	if errr != nil {
		return stringFormatSubruleScopes{}, regex, false, "", r.parseError(fmt.Sprintf("unable to compile %s as regexp", rule[1]), ruleNum, 1)
	}

	// Use custom error message if provided
	if len(rule) == 3 {
		errorMessage = rule[2]
	}
	return scopes, regex, negated, errorMessage, nil
}

// configError reports an invalid config, this is specifically the user's fault.
func (*StringFormatRule) configError(msg string, ruleNum, option int) error {
	return fmt.Errorf("invalid configuration for string-format: %s [argument %d, option %d]", msg, ruleNum, option)
}

// parseError reports a general config parsing failure, this may be the user's fault, but it isn't known for certain.
func (*StringFormatRule) parseError(msg string, ruleNum, option int) error {
	return fmt.Errorf("failed to parse configuration for string-format: %s [argument %d, option %d]", msg, ruleNum, option)
}

// parseScopeError reports a general scope config parsing failure, this may be the user's fault,
// but it isn't known for certain.
func (*StringFormatRule) parseScopeError(msg string, ruleNum, option, scopeNum int) error {
	return fmt.Errorf("failed to parse configuration for string-format: %s [argument %d, option %d, scope index %d]", msg, ruleNum, option, scopeNum)
}

func (w *lintStringFormatRule) Visit(node ast.Node) ast.Visitor {
	// First, check if node is a call expression
	call, ok := node.(*ast.CallExpr)
	if !ok {
		return w
	}

	// Get the name of the call expression to check against rule scope
	callName, ok := w.getCallName(call)
	if !ok {
		return w
	}

	for _, rule := range w.rules {
		for _, scope := range rule.scopes {
			if scope.funcName == callName {
				rule.apply(call, scope)
			}
		}
	}

	return w
}

// getCallName returns the name of a call expression in the form of package.Func or Func.
func (*lintStringFormatRule) getCallName(call *ast.CallExpr) (callName string, ok bool) {
	if ident, ok := call.Fun.(*ast.Ident); ok {
		// Local function call
		return ident.Name, true
	}

	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		// Scoped function call
		scope, ok := selector.X.(*ast.Ident)
		if ok {
			return scope.Name + "." + selector.Sel.Name, true
		}
		// Scoped function call inside structure
		recv, ok := selector.X.(*ast.SelectorExpr)
		if ok {
			return recv.Sel.Name + "." + selector.Sel.Name, true
		}
	}

	return "", false
}

// apply a single format rule to a call expression
// (should be done after verifying the that the call expression matches the rule's scope).
func (r *stringFormatSubrule) apply(call *ast.CallExpr, scope *stringFormatSubruleScope) {
	if len(call.Args) <= scope.argument {
		return
	}

	arg := call.Args[scope.argument]
	var lit *ast.BasicLit
	if scope.field != "" {
		// Try finding the scope's Field, treating arg as a composite literal
		composite, ok := arg.(*ast.CompositeLit)
		if !ok {
			return
		}
		for _, el := range composite.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != scope.field {
				continue
			}

			// We're now dealing with the exact field in the rule's scope, so if anything fails, we can safely return instead of continuing the loop
			lit, ok = kv.Value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return
			}
		}
	} else {
		var ok bool
		// Treat arg as a string literal
		lit, ok = arg.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
	}

	// extra safety check
	if lit == nil {
		return
	}

	// Unquote the string literal before linting
	unquoted := lit.Value[1 : len(lit.Value)-1]
	if r.stringIsOK(unquoted) {
		return
	}

	r.generateFailure(lit)
}

func (r *stringFormatSubrule) stringIsOK(s string) bool {
	matches := r.regexp.MatchString(s)
	if r.negated {
		return !matches
	}

	return matches
}

func (r *stringFormatSubrule) generateFailure(node ast.Node) {
	var failure string
	switch {
	case r.errorMessage != "":
		failure = r.errorMessage
	case r.negated:
		failure = fmt.Sprintf("string literal matches user defined regex /%s/", r.regexp.String())
	case !r.negated:
		failure = fmt.Sprintf("string literal doesn't match user defined regex /%s/", r.regexp.String())
	}

	r.onFailure(lint.Failure{
		Confidence: 1,
		Failure:    failure,
		Node:       node,
	})
}
