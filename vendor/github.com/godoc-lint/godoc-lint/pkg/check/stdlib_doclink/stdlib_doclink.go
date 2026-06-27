// Package stdlib_doclink provides a checker for detecting potential doc links
// to standard library symbols.
package stdlib_doclink

import (
	"fmt"
	gdc "go/doc/comment"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/godoc-lint/godoc-lint/pkg/check/stdlib_doclink/internal"
	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

// RequireStdlibDoclinkRule is the corresponding rule name.
const RequireStdlibDoclinkRule = model.RequireStdlibDoclinkRule

var ruleSet = model.RuleSet{}.Add(RequireStdlibDoclinkRule)

// StdlibDoclinkChecker checks for proper doc links to stdlib symbols.
type StdlibDoclinkChecker struct{}

// NewStdlibDoclinkChecker returns a new instance of the corresponding checker.
func NewStdlibDoclinkChecker() *StdlibDoclinkChecker {
	return &StdlibDoclinkChecker{}
}

// GetCoveredRules implements the corresponding interface method.
func (r *StdlibDoclinkChecker) GetCoveredRules() model.RuleSet {
	return ruleSet
}

// Apply implements the corresponding interface method.
func (r *StdlibDoclinkChecker) Apply(actx *model.AnalysisContext) error {
	includeTests := actx.Config.GetRuleOptions().RequireStdlibDoclinkIncludeTests

	docs := make(map[*model.CommentGroup]struct{}, 10*len(actx.InspectorResult.Files))

	for _, ir := range util.AnalysisApplicableFiles(actx, includeTests, model.RuleSet{}.Add(RequireStdlibDoclinkRule)) {
		if ir.PackageDoc != nil {
			docs[ir.PackageDoc] = struct{}{}
		}

		for _, sd := range ir.SymbolDecl {
			if sd.ParentDoc != nil {
				docs[sd.ParentDoc] = struct{}{}
			}
			if sd.Doc == nil {
				continue
			}
			docs[sd.Doc] = struct{}{}
		}
	}

	if len(docs) == 0 {
		return nil
	}

	stdlib := stdlib()
	pi := packageImports{
		importAsMap: make(map[string]string, 10),
		badImportAs: make(map[string]struct{}, 10),
	}

	for _, f := range actx.Pass.Files {
		ft := util.GetPassFileToken(f, actx.Pass)
		if ft == nil {
			continue
		}

		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			s, ok := stdlib[path]
			if !ok {
				// It's not a stdlib package.
				continue
			}

			var importedAs string
			if imp.Name != nil {
				alias := imp.Name.Name
				if alias == "" || alias == "." || alias == "_" {
					// We don't support _ or . imports.
					continue
				}
				importedAs = alias
			} else {
				importedAs = s.Name
			}

			if alreadyImportedPath, ok := pi.importAsMap[importedAs]; ok && alreadyImportedPath != path {
				// Different import paths with the same alias(es); we don't support this case.
				// It happens when people do aliasing like this:
				//
				// (foo.go)
				//   import blah "path/to/package"
				//
				// (bar.go)
				//   import blah "path/to/another/package"
				//
				// It's not inherently wrong, but due to the go doc tool's package-wide way of
				// resolving package names in doc links (i.e. pkg in [pkg.name] or [pkg.recv.name])
				// it'll not work in such collision cases. As a result, doc links with colliding
				// package aliases get rendered as plain text instead of links.
				pi.badImportAs[importedAs] = struct{}{}
				continue
			}

			pi.importAsMap[importedAs] = path
		}
	}

	for doc := range docs {
		checkStdlibDoclink(actx, &pi, doc)
	}
	return nil
}

type packageImports struct {
	importAsMap map[string]string
	badImportAs map[string]struct{}
}

func checkStdlibDoclink(
	actx *model.AnalysisContext,
	pi *packageImports,
	doc *model.CommentGroup,
) {
	if doc.DisabledRules.All || doc.DisabledRules.Rules.Has(RequireStdlibDoclinkRule) {
		return
	}

	applicableBlocks := make([]gdc.Block, 0, len(doc.Parsed.Content))
	for _, b := range doc.Parsed.Content {
		if _, ok := b.(*gdc.Code); ok {
			continue
		}
		// Doc links are not picked up in headings.
		if _, ok := b.(*gdc.Heading); ok {
			continue
		}
		applicableBlocks = append(applicableBlocks, b)
	}
	strippedCodeAndLinks := &gdc.Doc{
		Content: applicableBlocks,
	}
	text := string((&gdc.Printer{}).Comment(strippedCodeAndLinks))

	pds := findPotentialDoclinks(pi, text)
	if len(pds) == 0 {
		return
	}

	for _, pd := range pds {
		var count string
		if pd.count > 1 {
			count = fmt.Sprintf(" (%d instances)", pd.count)
		}

		actx.Pass.ReportRangef(&doc.CG, "text %q should be replaced with %q to link to stdlib %s%s", pd.originalNoStar, pd.doclink, kindTitle(pd.kind), count)
	}
}

func kindTitle(kind internal.SymbolKind) string {
	switch kind {
	case internal.SymbolKindType:
		return "type"
	case internal.SymbolKindFunc:
		return "function"
	case internal.SymbolKindVar:
		return "variable"
	case internal.SymbolKindConst:
		return "constant"
	case internal.SymbolKindMethod:
		return "method"
	default:
		return "symbol"
	}
}

type potentialDoclink struct {
	originalNoStar string // e.g. "encoding/json.Encoder" or "json.Encoder" (if imported as such)
	count          int
	doclink        string
	kind           internal.SymbolKind
}

var potentialDoclinkRE = regexp.MustCompile(`(?m)(?:^|\s)(\*?)([a-zA-Z_][a-zA-Z0-9_]*(?:/[a-zA-Z_][a-zA-Z0-9_]*)*)\.([a-zA-Z0-9_]+)(?:\.([a-zA-Z0-9_]+))?\b`)

func findPotentialDoclinks(pi *packageImports, text string) []*potentialDoclink {
	stdlib := stdlib()

	m := make(map[string]*potentialDoclink, 5)

	matches := potentialDoclinkRE.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		_ = match[1] // star, if any
		pkg := match[2]
		name1 := match[3]
		name2 := match[4]

		if pkg != "" && name1 != "" && name2 != "" {
			// pkg.recv.name (= pkg.name1.name2)

			path := pi.tryResolveImportPath(pkg)
			if path == "" {
				// Colliding import alias/name; skip.
				continue
			}

			s, ok := stdlib[path]
			if !ok {
				continue
			}

			kind, ok := s.Symbols[name1+"."+name2]
			if !ok {
				continue
			}

			originalNoStar := fmt.Sprintf("%s.%s.%s", pkg, name1, name2)
			if _, ok := m[originalNoStar]; !ok {
				doclink := fmt.Sprintf("[%s]", originalNoStar)
				m[originalNoStar] = &potentialDoclink{
					originalNoStar: originalNoStar,
					doclink:        doclink,
					kind:           kind,
				}
			}
			m[originalNoStar].count++
		} else if pkg != "" && name1 != "" && name2 == "" {
			// pkg.name (= pkg.name1)

			path := pi.tryResolveImportPath(pkg)
			if path == "" {
				// Colliding import alias/name; skip.
				continue
			}

			s, ok := stdlib[path]
			if !ok {
				continue
			}

			kind, ok := s.Symbols[name1]
			if !ok {
				continue
			}

			originalNoStar := fmt.Sprintf("%s.%s", pkg, name1)
			if _, ok := m[originalNoStar]; !ok {
				doclink := fmt.Sprintf("[%s]", originalNoStar)
				m[originalNoStar] = &potentialDoclink{
					originalNoStar: originalNoStar,
					doclink:        doclink,
					kind:           kind,
				}
			}
			m[originalNoStar].count++
		}
	}

	if len(m) == 0 {
		return nil
	}

	return slices.SortedFunc(maps.Values(m), func(a, b *potentialDoclink) int {
		return strings.Compare(a.originalNoStar, b.originalNoStar)
	})
}

// tryResolveImportPath tries to resolve the given package alias/name to its
// import path (package-wide lookup of imports). If the alias/name was among the
// existing imports, the method will return the underlying import path;
// otherwise it'll return the given argument as is.
//
// If the package alias/name is among bad/colliding imports, an empty string is
// returned.
//
// When spotting potential doc links like "pkg.name" or "pkg.recv.name", we
// should resolve the "pkg" part to the underlying import path. Not only that
// users can have a totally new/unknown alias for a stdlib package import, like:
//
//	import blah "encoding/json"
//
// they can also use another stdlib package's name as the alias, like:
//
//	import fmt "encoding/json"
//
// This is obviously rare, but still a valid use of language.
//
// Additionally, when importing slashed package paths, like "encoding/json",
// the package will be available as "json" by default; i.e.:
//
//	import "encoding/json"
//
// So, the actual package is not always evident from the import alias/name, so
// we need to carefully resolve the "pkg" part.
func (pi *packageImports) tryResolveImportPath(pkg string) string {
	path, ok := pi.importAsMap[pkg]
	if ok {
		if _, isBad := pi.badImportAs[pkg]; isBad {
			// We have different import paths with the same alias.
			return ""
		}
		return path
	}

	// Given "pkg" is not among existing imports.
	return pkg
}
