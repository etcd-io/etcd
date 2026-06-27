// Package inspect provides the pre-run inspection analyzer.
package inspect

import (
	"errors"
	"fmt"
	"go/ast"
	gdc "go/doc/comment"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const (
	metaName = "godoclint_inspect"
	metaDoc  = "Pre-run inspector for godoclint"
	metaURL  = "https://github.com/godoc-lint/godoc-lint"
)

// Inspector implements the godoc-lint pre-run inspector.
type Inspector struct {
	cb       model.ConfigBuilder
	exitFunc func(int, error)

	analyzer *analysis.Analyzer
	parser   gdc.Parser
}

// NewInspector returns a new instance of the inspector.
func NewInspector(cb model.ConfigBuilder, exitFunc func(int, error)) *Inspector {
	result := &Inspector{
		cb:       cb,
		exitFunc: exitFunc,
		analyzer: &analysis.Analyzer{
			Name:       metaName,
			Doc:        metaDoc,
			URL:        metaURL,
			ResultType: reflect.TypeFor[*model.InspectorResult](),
		},
	}
	result.analyzer.Run = result.run
	return result
}

// GetAnalyzer returns the underlying analyzer.
func (i *Inspector) GetAnalyzer() *analysis.Analyzer {
	return i.analyzer
}

var (
	topLevelOrphanCommentGroupPattern = regexp.MustCompile(`(?m)(?:^//.*\r?\n)+(?:\r?\n|\z)`)
	disableDirectivePattern           = regexp.MustCompile(`(?m)//godoclint:disable(?: *([^\r\n]+))?\r?$`)
)

func (i *Inspector) run(pass *analysis.Pass) (any, error) {
	if len(pass.Files) == 0 {
		return &model.InspectorResult{}, nil
	}

	ft := util.GetPassFileToken(pass.Files[0], pass)
	if ft == nil {
		err := errors.New("cannot prepare config")
		if i.exitFunc != nil {
			i.exitFunc(2, err)
		}
		return nil, err
	}

	pkgDir := filepath.Dir(ft.Name())
	cfg, err := i.cb.GetConfig(pkgDir)
	if err != nil {
		if i.exitFunc != nil {
			i.exitFunc(2, err)
		}
		return nil, err
	}

	inspect := func(f *ast.File) (*model.FileInspection, error) {
		ft := util.GetPassFileToken(f, pass)
		if ft == nil {
			return nil, nil
		}

		raw, err := pass.ReadFile(ft.Name())
		if err != nil {
			return nil, fmt.Errorf("cannot read file %q: %v", ft.Name(), err)
		}

		// Extract package godoc, if any.
		packageDoc := i.extractCommentGroup(f.Doc)

		// Extract top-level //godoclint:disable directives.
		disabledRules := model.InspectorResultDisableRules{}
		for _, match := range topLevelOrphanCommentGroupPattern.FindAll(raw, -1) {
			d := extractDisableDirectivesInComment(string(match))
			disabledRules.All = disabledRules.All || d.All
			disabledRules.Rules = disabledRules.Rules.Merge(d.Rules)
		}

		// Extract top-level symbol declarations.
		decls := make([]model.SymbolDecl, 0, len(f.Decls))
		for _, d := range f.Decls {
			switch dt := d.(type) {
			case *ast.FuncDecl:
				var recvBaseTypeName string
				var isMethod bool

				if dt.Recv != nil {
					isMethod = true
					if len(dt.Recv.List) > 0 {
						recvBaseTypeName = extractMethodRecvBaseTypeName(dt.Recv.List[0].Type)
					}
				}

				decls = append(decls, model.SymbolDecl{
					Decl:                   d,
					Kind:                   model.SymbolDeclKindFunc,
					Name:                   dt.Name.Name,
					Ident:                  dt.Name,
					IsMethod:               isMethod,
					MethodRecvBaseTypeName: recvBaseTypeName,
					Doc:                    i.extractCommentGroup(dt.Doc),
				})
			case *ast.BadDecl:
				decls = append(decls, model.SymbolDecl{
					Decl: d,
					Kind: model.SymbolDeclKindBad,
				})
			case *ast.GenDecl:
				switch dt.Tok {
				case token.CONST, token.VAR:
					kind := model.SymbolDeclKindConst
					if dt.Tok == token.VAR {
						kind = model.SymbolDeclKindVar
					}
					if dt.Lparen == token.NoPos {
						// cases:
						// const ... (single line)
						// var ... (single line)

						spec := dt.Specs[0].(*ast.ValueSpec)
						if len(spec.Names) == 1 {
							// cases:
							// const foo = 0
							// var foo = 0
							decls = append(decls, model.SymbolDecl{
								Decl:        d,
								Kind:        kind,
								Name:        spec.Names[0].Name,
								Ident:       spec.Names[0],
								Doc:         i.extractCommentGroup(dt.Doc),
								TrailingDoc: i.extractCommentGroup(spec.Comment),
							})
						} else {
							// cases:
							// const foo, bar = 0, 0
							// var foo, bar = 0, 0
							doc := i.extractCommentGroup(dt.Doc)
							trailingDoc := i.extractCommentGroup(spec.Comment)
							for ix, n := range spec.Names {
								decls = append(decls, model.SymbolDecl{
									Decl:           d,
									Kind:           kind,
									Name:           n.Name,
									Ident:          n,
									Doc:            doc,
									TrailingDoc:    trailingDoc,
									MultiNameDecl:  true,
									MultiNameIndex: ix,
								})
							}
						}
					} else {
						// cases:
						// const (
						//     foo = 0
						// )
						// var (
						//     foo = 0
						// )
						// const (
						//     foo, bar = 0, 0
						// )
						// var (
						//     foo, bar = 0, 0
						// )

						parentDoc := i.extractCommentGroup(dt.Doc)
						for spix, s := range dt.Specs {
							spec := s.(*ast.ValueSpec)
							doc := i.extractCommentGroup(spec.Doc)
							trailingDoc := i.extractCommentGroup(spec.Comment)
							for ix, n := range spec.Names {
								decls = append(decls, model.SymbolDecl{
									Decl:           d,
									Kind:           kind,
									Name:           n.Name,
									Ident:          n,
									Doc:            doc,
									TrailingDoc:    trailingDoc,
									ParentDoc:      parentDoc,
									MultiNameDecl:  len(spec.Names) > 1,
									MultiNameIndex: ix,
									MultiSpecDecl:  true,
									MultiSpecIndex: spix,
								})
							}
						}
					}
				case token.TYPE:
					if dt.Lparen == token.NoPos {
						// case:
						// type foo int

						spec := dt.Specs[0].(*ast.TypeSpec)
						decls = append(decls, model.SymbolDecl{
							Decl:        d,
							Kind:        model.SymbolDeclKindType,
							IsTypeAlias: spec.Assign != token.NoPos,
							Name:        spec.Name.Name,
							Ident:       spec.Name,
							Doc:         i.extractCommentGroup(dt.Doc),
							TrailingDoc: i.extractCommentGroup(spec.Comment),
						})
					} else {
						// case:
						// type (
						//     foo int
						// )

						parentDoc := i.extractCommentGroup(dt.Doc)
						for spix, s := range dt.Specs {
							spec := s.(*ast.TypeSpec)
							decls = append(decls, model.SymbolDecl{
								Decl:           d,
								Kind:           model.SymbolDeclKindType,
								IsTypeAlias:    spec.Assign != token.NoPos,
								Name:           spec.Name.Name,
								Ident:          spec.Name,
								Doc:            i.extractCommentGroup(spec.Doc),
								TrailingDoc:    i.extractCommentGroup(spec.Comment),
								ParentDoc:      parentDoc,
								MultiSpecDecl:  true,
								MultiSpecIndex: spix,
							})
						}
					}
				default:
					continue
				}
			}
		}

		return &model.FileInspection{
			DisabledRules: disabledRules,
			PackageDoc:    packageDoc,
			SymbolDecl:    decls,
		}, nil
	}

	result := &model.InspectorResult{
		Files: make(map[*ast.File]*model.FileInspection, len(pass.Files)),
	}

	for _, f := range pass.Files {
		ft := util.GetPassFileToken(f, pass)
		if ft == nil {
			continue
		}
		if !cfg.IsPathApplicable(ft.Name()) {
			continue
		}

		if fi, err := inspect(f); err != nil {
			return nil, fmt.Errorf("inspector failed: %w", err)
		} else {
			result.Files[f] = fi
		}
	}
	return result, nil
}

func (i *Inspector) extractCommentGroup(cg *ast.CommentGroup) *model.CommentGroup {
	if cg == nil {
		return nil
	}

	lines := make([]string, 0, len(cg.List))
	for _, l := range cg.List {
		lines = append(lines, l.Text)
	}
	rawText := strings.Join(lines, "\n")

	text := cg.Text()
	return &model.CommentGroup{
		CG:            *cg,
		Parsed:        *i.parser.Parse(text),
		Text:          text,
		DisabledRules: extractDisableDirectivesInComment(rawText),
	}
}

func extractDisableDirectivesInComment(s string) model.InspectorResultDisableRules {
	result := model.InspectorResultDisableRules{}
	for _, directive := range disableDirectivePattern.FindAllStringSubmatch(s, -1) {
		args := directive[1]
		if args == "" {
			result.All = true
			continue
		}

		for name := range strings.SplitSeq(strings.TrimSpace(args), " ") {
			if model.AllRules.Has(model.Rule(name)) {
				result.Rules = result.Rules.Add(model.Rule(name))
			}
		}
	}
	return result
}

func extractMethodRecvBaseTypeName(expr ast.Expr) string {
	switch tt := expr.(type) {
	case *ast.Ident:
		return tt.Name
	case *ast.StarExpr:
		return extractMethodRecvBaseTypeName(tt.X)
	case *ast.IndexExpr:
		return extractMethodRecvBaseTypeName(tt.X)
	case *ast.IndexListExpr:
		return extractMethodRecvBaseTypeName(tt.X)
	}
	return ""
}
