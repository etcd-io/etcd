package exhaustive

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

func init() {
	registerFlags()
}

func registerFlags() {
	Analyzer.Flags.Var(&fCheck, CheckFlag, "comma-separated list of program `elements` to check for exhaustiveness; supported element values: switch, map")
	Analyzer.Flags.BoolVar(&fExplicitExhaustiveSwitch, ExplicitExhaustiveSwitchFlag, false, `check switch statement only if associated with "//exhaustive:enforce" comment`)
	Analyzer.Flags.BoolVar(&fExplicitExhaustiveMap, ExplicitExhaustiveMapFlag, false, `check map literal only if associated with "//exhaustive:enforce" comment`)
	Analyzer.Flags.BoolVar(&fCheckGenerated, CheckGeneratedFlag, false, "check generated files")
	Analyzer.Flags.BoolVar(&fDefaultSignifiesExhaustive, DefaultSignifiesExhaustiveFlag, false, "switch statement is unconditionally exhaustive if it has a default case")
	Analyzer.Flags.BoolVar(&fDefaultCaseRequired, DefaultCaseRequiredFlag, false, "switch statement requires default case even if exhaustive")
	Analyzer.Flags.Var(&fIgnoreEnumMembers, IgnoreEnumMembersFlag, "ignore constants matching `regexp`")
	Analyzer.Flags.Var(&fIgnoreEnumTypes, IgnoreEnumTypesFlag, "ignore types matching `regexp`")
	Analyzer.Flags.BoolVar(&fPackageScopeOnly, PackageScopeOnlyFlag, false, "only discover enums declared in file-level blocks")

	var unused string
	Analyzer.Flags.StringVar(&unused, IgnorePatternFlag, "", "no effect (deprecated); use -"+IgnoreEnumMembersFlag)
	Analyzer.Flags.StringVar(&unused, CheckingStrategyFlag, "", "no effect (deprecated)")
}

// Flag names used by the analyzer. These are exported for use by analyzer
// driver programs.
const (
	CheckFlag                      = "check"
	ExplicitExhaustiveSwitchFlag   = "explicit-exhaustive-switch"
	ExplicitExhaustiveMapFlag      = "explicit-exhaustive-map"
	CheckGeneratedFlag             = "check-generated"
	DefaultSignifiesExhaustiveFlag = "default-signifies-exhaustive"
	DefaultCaseRequiredFlag        = "default-case-required"
	IgnoreEnumMembersFlag          = "ignore-enum-members"
	IgnoreEnumTypesFlag            = "ignore-enum-types"
	PackageScopeOnlyFlag           = "package-scope-only"

	// Deprecated flag names.
	IgnorePatternFlag    = "ignore-pattern"    // Deprecated: use IgnoreEnumMembersFlag.
	CheckingStrategyFlag = "checking-strategy" // Deprecated: no longer applicable.
)

// Flag values.
var (
	fCheck                      = stringsFlag{elements: defaultCheckElements, filter: validCheckElement}
	fExplicitExhaustiveSwitch   bool
	fExplicitExhaustiveMap      bool
	fCheckGenerated             bool
	fDefaultSignifiesExhaustive bool
	fDefaultCaseRequired        bool
	fIgnoreEnumMembers          regexpFlag
	fIgnoreEnumTypes            regexpFlag
	fPackageScopeOnly           bool
)

// resetFlags resets the flag variables to default values.
// Useful in tests.
func resetFlags() {
	fCheck = stringsFlag{elements: defaultCheckElements, filter: validCheckElement}
	fExplicitExhaustiveSwitch = false
	fExplicitExhaustiveMap = false
	fCheckGenerated = false
	fDefaultSignifiesExhaustive = false
	fDefaultCaseRequired = false
	fIgnoreEnumMembers = regexpFlag{}
	fIgnoreEnumTypes = regexpFlag{}
	fPackageScopeOnly = false
}

// checkElement is a program element supported by the -check flag.
type checkElement string

const (
	elementSwitch checkElement = "switch"
	elementMap    checkElement = "map"
)

func validCheckElement(s string) error {
	switch checkElement(s) {
	case elementSwitch:
		return nil
	case elementMap:
		return nil
	default:
		return fmt.Errorf("invalid program element %q", s)
	}
}

var defaultCheckElements = []string{
	string(elementSwitch),
}

var Analyzer = &analysis.Analyzer{
	Name:      "exhaustive",
	Doc:       "check exhaustiveness of enum switch statements",
	Run:       run,
	Requires:  []*analysis.Analyzer{inspect.Analyzer},
	FactTypes: []analysis.Fact{&enumMembersFact{}},
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	for typ, members := range findEnums(fPackageScopeOnly, pass.Pkg, inspect, pass.TypesInfo) {
		exportFact(pass, typ, members)
	}

	generated := boolCache{compute: isGeneratedFile}
	comments := commentCache{compute: fileCommentMap}

	// NOTE: should not share the same inspect.WithStack call for different
	// program elements: the visitor function for a program element may
	// exit traversal early, but this shouldn't affect traversal for
	// other program elements.
	for _, e := range fCheck.elements {
		switch checkElement(e) {
		case elementSwitch:
			conf := switchConfig{
				explicit:                   fExplicitExhaustiveSwitch,
				defaultSignifiesExhaustive: fDefaultSignifiesExhaustive,
				defaultCaseRequired:        fDefaultCaseRequired,
				checkGenerated:             fCheckGenerated,
				ignoreConstant:             fIgnoreEnumMembers.re,
				ignoreType:                 fIgnoreEnumTypes.re,
			}
			checker := switchChecker(pass, conf, generated, comments)
			inspect.WithStack([]ast.Node{&ast.SwitchStmt{}}, toVisitor(checker))

		case elementMap:
			conf := mapConfig{
				explicit:       fExplicitExhaustiveMap,
				checkGenerated: fCheckGenerated,
				ignoreConstant: fIgnoreEnumMembers.re,
				ignoreType:     fIgnoreEnumTypes.re,
			}
			checker := mapChecker(pass, conf, generated, comments)
			inspect.WithStack([]ast.Node{&ast.CompositeLit{}}, toVisitor(checker))

		default:
			panic(fmt.Sprintf("unknown checkElement %v", e))
		}
	}

	return nil, nil
}
