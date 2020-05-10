// keyify transforms unkeyed struct literals into a keyed ones.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"

	"honnef.co/go/tools/version"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/buildutil"
	"golang.org/x/tools/go/loader"
)

var (
	fRecursive bool
	fOneLine   bool
	fJSON      bool
	fMinify    bool
	fModified  bool
	fVersion   bool
)

func init() {
	flag.BoolVar(&fRecursive, "r", false, "keyify struct initializers recursively")
	flag.BoolVar(&fOneLine, "o", false, "print new struct initializer on a single line")
	flag.BoolVar(&fJSON, "json", false, "print new struct initializer as JSON")
	flag.BoolVar(&fMinify, "m", false, "omit fields that are set to their zero value")
	flag.BoolVar(&fModified, "modified", false, "read an archive of modified files from standard input")
	flag.BoolVar(&fVersion, "version", false, "Print version and exit")
}

func usage() {
	fmt.Printf("Usage: %s [flags] <position>\n\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	if fVersion {
		version.Print()
		os.Exit(0)
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	pos := flag.Args()[0]
	name, start, _, err := parsePos(pos)
	if err != nil {
		log.Fatal(err)
	}
	eval, err := filepath.EvalSymlinks(name)
	if err != nil {
		log.Fatal(err)
	}
	name, err = filepath.Abs(eval)
	if err != nil {
		log.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	ctx := &build.Default
	if fModified {
		overlay, err := buildutil.ParseOverlayArchive(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		ctx = buildutil.OverlayContext(ctx, overlay)
	}
	bpkg, err := buildutil.ContainingPackage(ctx, cwd, name)
	if err != nil {
		log.Fatal(err)
	}
	conf := &loader.Config{
		Build: ctx,
	}
	conf.TypeCheckFuncBodies = func(s string) bool {
		return s == bpkg.ImportPath || s == bpkg.ImportPath+"_test"
	}
	conf.ImportWithTests(bpkg.ImportPath)
	lprog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}
	var tf *token.File
	var af *ast.File
	var pkg *loader.PackageInfo
outer:
	for _, pkg = range lprog.InitialPackages() {
		for _, ff := range pkg.Files {
			file := lprog.Fset.File(ff.Pos())
			if file.Name() == name {
				af = ff
				tf = file
				break outer
			}
		}
	}
	if tf == nil {
		log.Fatalf("couldn't find file %s", name)
	}
	tstart, tend, err := fileOffsetToPos(tf, start, start)
	if err != nil {
		log.Fatal(err)
	}
	path, _ := astutil.PathEnclosingInterval(af, tstart, tend)
	var complit *ast.CompositeLit
	for _, p := range path {
		if p, ok := p.(*ast.CompositeLit); ok {
			complit = p
			break
		}
	}
	if complit == nil {
		log.Fatal("no composite literal found near point")
	}
	if len(complit.Elts) == 0 {
		printComplit(complit, complit, lprog.Fset, lprog.Fset)
		return
	}
	if _, ok := complit.Elts[0].(*ast.KeyValueExpr); ok {
		lit := complit
		if fOneLine {
			lit = copyExpr(complit, 1).(*ast.CompositeLit)
		}
		printComplit(complit, lit, lprog.Fset, lprog.Fset)
		return
	}
	_, ok := pkg.TypeOf(complit).Underlying().(*types.Struct)
	if !ok {
		log.Fatal("not a struct initialiser")
		return
	}

	newComplit, lines := keyify(pkg, complit)
	newFset := token.NewFileSet()
	newFile := newFset.AddFile("", -1, lines)
	for i := 1; i <= lines; i++ {
		newFile.AddLine(i)
	}
	printComplit(complit, newComplit, lprog.Fset, newFset)
}

func keyify(
	pkg *loader.PackageInfo,
	complit *ast.CompositeLit,
) (*ast.CompositeLit, int) {
	var calcPos func(int) token.Pos
	if fOneLine {
		calcPos = func(int) token.Pos { return token.Pos(1) }
	} else {
		calcPos = func(i int) token.Pos { return token.Pos(2 + i) }
	}

	st, _ := pkg.TypeOf(complit).Underlying().(*types.Struct)
	newComplit := &ast.CompositeLit{
		Type:   complit.Type,
		Lbrace: 1,
		Rbrace: token.Pos(st.NumFields() + 2),
	}
	if fOneLine {
		newComplit.Rbrace = 1
	}
	numLines := 2 + st.NumFields()
	n := 0
	for i := 0; i < st.NumFields(); i++ {
		field := st.Field(i)
		val := complit.Elts[i]
		if fRecursive {
			if val2, ok := val.(*ast.CompositeLit); ok {
				if _, ok := pkg.TypeOf(val2.Type).Underlying().(*types.Struct); ok {
					// FIXME(dh): this code is obviously wrong. But
					// what were we intending to do here?
					var lines int
					numLines += lines
					//lint:ignore SA4006 See FIXME above.
					val, lines = keyify(pkg, val2)
				}
			}
		}
		_, isIface := st.Field(i).Type().Underlying().(*types.Interface)
		if fMinify && (isNil(val, pkg) || (!isIface && isZero(val, pkg))) {
			continue
		}
		elt := &ast.KeyValueExpr{
			Key:   &ast.Ident{NamePos: calcPos(n), Name: field.Name()},
			Value: copyExpr(val, calcPos(n)),
		}
		newComplit.Elts = append(newComplit.Elts, elt)
		n++
	}
	return newComplit, numLines
}

func isNil(val ast.Expr, pkg *loader.PackageInfo) bool {
	ident, ok := val.(*ast.Ident)
	if !ok {
		return false
	}
	if _, ok := pkg.ObjectOf(ident).(*types.Nil); ok {
		return true
	}
	if c, ok := pkg.ObjectOf(ident).(*types.Const); ok {
		if c.Val().Kind() != constant.Bool {
			return false
		}
		return !constant.BoolVal(c.Val())
	}
	return false
}

func isZero(val ast.Expr, pkg *loader.PackageInfo) bool {
	switch val := val.(type) {
	case *ast.BasicLit:
		switch val.Value {
		case `""`, "``", "0", "0.0", "0i", "0.":
			return true
		default:
			return false
		}
	case *ast.Ident:
		return isNil(val, pkg)
	case *ast.CompositeLit:
		typ := pkg.TypeOf(val.Type)
		if typ == nil {
			return false
		}
		isIface := false
		switch typ := typ.Underlying().(type) {
		case *types.Struct:
		case *types.Array:
			_, isIface = typ.Elem().Underlying().(*types.Interface)
		default:
			return false
		}
		for _, elt := range val.Elts {
			if isNil(elt, pkg) || (!isIface && !isZero(elt, pkg)) {
				return false
			}
		}
		return true
	}
	return false
}

func printComplit(oldlit, newlit *ast.CompositeLit, oldfset, newfset *token.FileSet) {
	buf := &bytes.Buffer{}
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	_ = cfg.Fprint(buf, newfset, newlit)
	if fJSON {
		output := struct {
			Start       int    `json:"start"`
			End         int    `json:"end"`
			Replacement string `json:"replacement"`
		}{
			oldfset.Position(oldlit.Pos()).Offset,
			oldfset.Position(oldlit.End()).Offset,
			buf.String(),
		}
		_ = json.NewEncoder(os.Stdout).Encode(output)
	} else {
		fmt.Println(buf.String())
	}
}

func copyExpr(expr ast.Expr, line token.Pos) ast.Expr {
	switch expr := expr.(type) {
	case *ast.BasicLit:
		cp := *expr
		cp.ValuePos = 0
		return &cp
	case *ast.BinaryExpr:
		cp := *expr
		cp.X = copyExpr(cp.X, line)
		cp.OpPos = 0
		cp.Y = copyExpr(cp.Y, line)
		return &cp
	case *ast.CallExpr:
		cp := *expr
		cp.Fun = copyExpr(cp.Fun, line)
		cp.Lparen = 0
		for i, v := range cp.Args {
			cp.Args[i] = copyExpr(v, line)
		}
		if cp.Ellipsis != 0 {
			cp.Ellipsis = line
		}
		cp.Rparen = 0
		return &cp
	case *ast.CompositeLit:
		cp := *expr
		cp.Type = copyExpr(cp.Type, line)
		cp.Lbrace = 0
		for i, v := range cp.Elts {
			cp.Elts[i] = copyExpr(v, line)
		}
		cp.Rbrace = 0
		return &cp
	case *ast.Ident:
		cp := *expr
		cp.NamePos = 0
		return &cp
	case *ast.IndexExpr:
		cp := *expr
		cp.X = copyExpr(cp.X, line)
		cp.Lbrack = 0
		cp.Index = copyExpr(cp.Index, line)
		cp.Rbrack = 0
		return &cp
	case *ast.KeyValueExpr:
		cp := *expr
		cp.Key = copyExpr(cp.Key, line)
		cp.Colon = 0
		cp.Value = copyExpr(cp.Value, line)
		return &cp
	case *ast.ParenExpr:
		cp := *expr
		cp.Lparen = 0
		cp.X = copyExpr(cp.X, line)
		cp.Rparen = 0
		return &cp
	case *ast.SelectorExpr:
		cp := *expr
		cp.X = copyExpr(cp.X, line)
		cp.Sel = copyExpr(cp.Sel, line).(*ast.Ident)
		return &cp
	case *ast.SliceExpr:
		cp := *expr
		cp.X = copyExpr(cp.X, line)
		cp.Lbrack = 0
		cp.Low = copyExpr(cp.Low, line)
		cp.High = copyExpr(cp.High, line)
		cp.Max = copyExpr(cp.Max, line)
		cp.Rbrack = 0
		return &cp
	case *ast.StarExpr:
		cp := *expr
		cp.Star = 0
		cp.X = copyExpr(cp.X, line)
		return &cp
	case *ast.TypeAssertExpr:
		cp := *expr
		cp.X = copyExpr(cp.X, line)
		cp.Lparen = 0
		cp.Type = copyExpr(cp.Type, line)
		cp.Rparen = 0
		return &cp
	case *ast.UnaryExpr:
		cp := *expr
		cp.OpPos = 0
		cp.X = copyExpr(cp.X, line)
		return &cp
	case *ast.MapType:
		cp := *expr
		cp.Map = 0
		cp.Key = copyExpr(cp.Key, line)
		cp.Value = copyExpr(cp.Value, line)
		return &cp
	case *ast.ArrayType:
		cp := *expr
		cp.Lbrack = 0
		cp.Len = copyExpr(cp.Len, line)
		cp.Elt = copyExpr(cp.Elt, line)
		return &cp
	case *ast.Ellipsis:
		cp := *expr
		cp.Elt = copyExpr(cp.Elt, line)
		cp.Ellipsis = line
		return &cp
	case *ast.InterfaceType:
		cp := *expr
		cp.Interface = 0
		return &cp
	case *ast.StructType:
		cp := *expr
		cp.Struct = 0
		return &cp
	case *ast.FuncLit:
		return expr
	case *ast.ChanType:
		cp := *expr
		cp.Arrow = 0
		cp.Begin = 0
		cp.Value = copyExpr(cp.Value, line)
		return &cp
	case nil:
		return nil
	default:
		panic(fmt.Sprintf("shouldn't happen: unknown ast.Expr of type %T", expr))
	}
}
