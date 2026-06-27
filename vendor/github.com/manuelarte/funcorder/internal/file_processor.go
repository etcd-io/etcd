package internal

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// FileProcessor Holder to store all the functions that are potential to be constructors and all the structs.
type FileProcessor struct {
	structs       map[string]*StructHolder
	features      Feature
	topLevelFuncs []*ast.FuncDecl
}

// NewFileProcessor creates a new file processor.
func NewFileProcessor(checkers Feature) *FileProcessor {
	return &FileProcessor{
		structs:  make(map[string]*StructHolder),
		features: checkers,
	}
}

// Analyze check whether the order of the methods in the constructor is correct.
func (fp *FileProcessor) Analyze(pass *analysis.Pass) {
	for _, sh := range fp.structs {
		// filter out structs that are not declared inside that file
		if sh.Struct != nil {
			sh.Analyze(pass)
		}
	}

	if fp.features.IsEnabled(FunctionCheck) {
		fp.analyzeFunctions(pass)
	}
}

func (fp *FileProcessor) ResetStructs() {
	fp.structs = make(map[string]*StructHolder)
	fp.topLevelFuncs = nil
}

func (fp *FileProcessor) AddFuncDecl(n *ast.FuncDecl) {
	if fp.features.IsEnabled(FunctionCheck) && n.Recv == nil {
		fp.topLevelFuncs = append(fp.topLevelFuncs, n)
	}

	if sc := NewStructConstructor(n); sc != nil {
		sh := fp.getOrCreate(sc.StructReturn.Name)
		sh.Constructors = append(sh.Constructors, sc.Constructor)

		return
	}

	if st := funcIsMethod(n); st != nil {
		sh := fp.getOrCreate(st.Name)
		sh.StructMethods = append(sh.StructMethods, n)
	}
}

func (fp *FileProcessor) AddTypeSpec(n *ast.TypeSpec) {
	sh := fp.getOrCreate(n.Name.Name)
	sh.Struct = n
}

// analyzeFunctions reports every unexported top-level function that appears
// before the last exported top-level function in source order.
// The `init` function is excluded from this check.
func (fp *FileProcessor) analyzeFunctions(pass *analysis.Pass) {
	var lastExported *ast.FuncDecl

	for _, fn := range fp.topLevelFuncs {
		if fn.Name.Name == "init" {
			continue
		}

		if !fn.Name.IsExported() {
			continue
		}

		if lastExported == nil || fn.Pos() > lastExported.Pos() {
			lastExported = fn
		}
	}

	if lastExported == nil {
		return
	}

	for _, fn := range fp.topLevelFuncs {
		if fn.Name.Name == "init" {
			continue
		}

		if fn.Name.IsExported() || fn.Pos() >= lastExported.Pos() {
			continue
		}

		reportUnexportedFuncBeforeExportedFunc(pass, fn, lastExported)
	}
}

func (fp *FileProcessor) getOrCreate(structName string) *StructHolder {
	if holder, ok := fp.structs[structName]; ok {
		return holder
	}

	created := &StructHolder{
		Features: fp.features,
	}
	fp.structs[structName] = created

	return created
}

func funcIsMethod(n *ast.FuncDecl) *ast.Ident {
	if n.Recv == nil {
		return nil
	}

	if len(n.Recv.List) != 1 {
		return nil
	}

	return getIdent(n.Recv.List[0].Type)
}

func getIdent(expr ast.Expr) *ast.Ident {
	switch exp := expr.(type) {
	case *ast.StarExpr:
		return getIdent(exp.X)

	case *ast.Ident:
		return exp

	default:
		return nil
	}
}
