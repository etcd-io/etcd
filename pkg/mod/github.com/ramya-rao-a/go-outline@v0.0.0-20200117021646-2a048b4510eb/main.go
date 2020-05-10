package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"

	"golang.org/x/tools/go/buildutil"
)

type Declaration struct {
	Label        string        `json:"label"`
	Type         string        `json:"type"`
	ReceiverType string        `json:"receiverType,omitempty"`
	Start        token.Pos     `json:"start"`
	End          token.Pos     `json:"end"`
	Children     []Declaration `json:"children,omitempty"`
}

var (
	file        = flag.String("f", "", "the path to the file to outline")
	importsOnly = flag.Bool("imports-only", false, "parse imports only")
	modified    = flag.Bool("modified", false, "read an archive of the modified file from standard input")
)

func main() {
	flag.Parse()
	fset := token.NewFileSet()
	parserMode := parser.ParseComments
	if *importsOnly == true {
		parserMode = parser.ImportsOnly
	}

	var fileAst *ast.File
	var err error

	if *modified == true {
		archive, err := buildutil.ParseOverlayArchive(os.Stdin)
		if err != nil {
			reportError(fmt.Errorf("failed to parse -modified archive: %v", err))
		}
		fc, ok := archive[*file]
		if !ok {
			reportError(fmt.Errorf("couldn't find %s in archive", *file))
		}
		fileAst, err = parser.ParseFile(fset, *file, fc, parserMode)
	} else {
		fileAst, err = parser.ParseFile(fset, *file, nil, parserMode)
	}

	if err != nil {
		reportError(fmt.Errorf("Could not parse file %s", *file))
	}

	declarations := []Declaration{}

	for _, decl := range fileAst.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			receiverType, err := getReceiverType(fset, decl)
			if err != nil {
				reportError(fmt.Errorf("Failed to parse receiver type: %v", err))
			}
			declarations = append(declarations, Declaration{
				decl.Name.String(),
				"function",
				receiverType,
				decl.Pos(),
				decl.End(),
				[]Declaration{},
			})
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.ImportSpec:
					declarations = append(declarations, Declaration{
						spec.Path.Value,
						"import",
						"",
						spec.Pos(),
						spec.End(),
						[]Declaration{},
					})
				case *ast.TypeSpec:
					//TODO: Members if it's a struct or interface type?
					declarations = append(declarations, Declaration{
						spec.Name.String(),
						"type",
						"",
						spec.Pos(),
						spec.End(),
						[]Declaration{},
					})
				case *ast.ValueSpec:
					for _, id := range spec.Names {
						varOrConst := "variable"
						if decl.Tok == token.CONST {
							varOrConst = "constant"
						}
						declarations = append(declarations, Declaration{
							id.Name,
							varOrConst,
							"",
							id.Pos(),
							id.End(),
							[]Declaration{},
						})
					}
				default:
					reportError(fmt.Errorf("Unknown token type: %s", decl.Tok))
				}
			}
		default:
			reportError(fmt.Errorf("Unknown declaration @ %v", decl.Pos()))
		}
	}

	pkg := []*Declaration{&Declaration{
		fileAst.Name.String(),
		"package",
		"",
		fileAst.Pos(),
		fileAst.End(),
		declarations,
	}}

	str, _ := json.Marshal(pkg)
	fmt.Println(string(str))

}

func getReceiverType(fset *token.FileSet, decl *ast.FuncDecl) (string, error) {
	if decl.Recv == nil {
		return "", nil
	}

	buf := &bytes.Buffer{}
	if err := format.Node(buf, fset, decl.Recv.List[0].Type); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func reportError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
}
