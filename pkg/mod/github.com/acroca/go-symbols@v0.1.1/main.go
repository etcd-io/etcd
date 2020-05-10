// The gosymbols command prints type information for package-level symbols.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/buildutil"
)

const usage = `Usage: gosymbols <package> ...`

var ignoreFoldersString string
var ignoreFolders []string

func init() {
	flag.Var((*buildutil.TagsFlag)(&build.Default.BuildTags), "tags", buildutil.TagsFlagDoc)
	flag.StringVar(&ignoreFoldersString, "ignore", "", "a comma-separated list of folders to ignore.")
}

func main() {
	if err := doMain(); err != nil {
		fmt.Fprintf(os.Stderr, "go-symbols: %s\n", err)
		os.Exit(1)
	}
}

type symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Package   string `json:"package"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

var mutex sync.Mutex
var syms = make([]symbol, 0)

type visitor struct {
	pkg   *ast.Package
	fset  *token.FileSet
	query string
	syms  []symbol
}

func (v *visitor) Visit(node ast.Node) bool {
	descend := true

	var ident *ast.Ident
	var kind string
	var structName string

	switch t := node.(type) {
	case *ast.FuncDecl:
		kind = "func"
		ident = t.Name
		descend = false
		// Adding struct(class) name before function name if it's struct method
		if t.Recv != nil {
			// if function is method it hast 1 member in Recv list with type ident
			if len(t.Recv.List) == 1 {
				switch xt := t.Recv.List[0].Type.(type) {
				case *ast.Ident: // in case if it's plain struct
					structName = xt.Name + "."
				case *ast.StarExpr: // in case it's pointer to struct
					switch xt2 := xt.X.(type) {
					case *ast.Ident:
						structName = xt2.Name + "."
					}
				}

			}
		}

	case *ast.TypeSpec:
		kind = "type"
		ident = t.Name
		descend = false
	}

	if ident != nil && strings.Contains(strings.ToLower(ident.Name), v.query) {
		f := v.fset.File(ident.Pos())
		v.syms = append(v.syms, symbol{
			Package: v.pkg.Name,
			Path:    f.Name(),
			Name:    structName + ident.Name,
			Kind:    kind,
			Line:    f.Line(ident.Pos()) - 1,
		})
	}

	return descend
}

var haveSrcDir = true

func forEachPackage(ctxt *build.Context, found func(importPath string, err error)) {
	// We use a counting semaphore to limit
	// the number of parallel calls to ReadDir.
	sema := make(chan bool, 20)

	ch := make(chan item)

	var srcDirs []string
	if haveSrcDir {
		srcDirs = ctxt.SrcDirs()
	} else {
		srcDirs = append(srcDirs, ctxt.GOPATH)
	}

	var wg sync.WaitGroup
	for _, root := range srcDirs {
		root := root
		wg.Add(1)
		go func() {
			allPackages(ctxt, sema, root, ch)
			wg.Done()
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	// All calls to found occur in the caller's goroutine.
	for i := range ch {
		found(i.importPath, i.err)
	}
}

type item struct {
	importPath string
	err        error // (optional)
}

func allPackages(ctxt *build.Context, sema chan bool, root string, ch chan<- item) {
	root = filepath.Clean(root) + string(os.PathSeparator)

	var wg sync.WaitGroup

	var walkDir func(dir string)
	walkDir = func(dir string) {
		// Avoid .foo, _foo, and testdata directory trees.
		base := filepath.Base(dir)
		if base == "" || base[0] == '.' || base[0] == '_' || base == "testdata" {
			return
		}
		for _, ignoredFolder := range ignoreFolders {
			if base == ignoredFolder {
				return
			}
		}

		pkg := filepath.ToSlash(strings.TrimPrefix(dir, root))

		// Prune search if we encounter any of these import paths.
		switch pkg {
		case "builtin":
			return
		}

		sema <- true
		files, err := ioutil.ReadDir(dir)
		<-sema

		if err == nil {
			ch <- item{pkg, err}
		}
		for _, fi := range files {
			fi := fi
			if fi.IsDir() {
				wg.Add(1)
				go func() {
					walkDir(filepath.Join(dir, fi.Name()))
					wg.Done()
				}()
			}
		}
	}

	walkDir(root)
	wg.Wait()
}

func doMain() error {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.Args()

	if ignoreFoldersString != "" {
		ignoreFolders = strings.Split(ignoreFoldersString, ",")
	}

	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	dir := args[0]
	var query string
	if len(args) > 1 {
		query = args[1]
	}
	query = strings.ToLower(query)

	ctxt := build.Default // copy
	ctxt.GOPATH = dir     // disable GOPATH
	ctxt.GOROOT = ""

	fset := token.NewFileSet()
	sema := make(chan int, 8) // concurrency-limiting semaphore
	var wg sync.WaitGroup

	if _, err := os.Stat(filepath.Join(dir, "src")); err != nil {
		haveSrcDir = false
	}

	// Here we can't use buildutil.ForEachPackage here since it only considers
	// src dirs and this tool should be able to run against a golang source dir.
	forEachPackage(&ctxt, func(path string, err error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sema <- 1 // acquire token
			defer func() {
				<-sema // release token
			}()

			v := &visitor{
				fset:  fset,
				query: query,
			}

			if haveSrcDir {
				path = filepath.Join(dir, "src", path)
			} else {
				path = filepath.Join(dir, path)
			}

			parsed, _ := parser.ParseDir(fset, path, nil, 0)
			// Ignore any errors, they are irrelevant for symbol search.

			for _, astpkg := range parsed {
				v.pkg = astpkg
				for _, f := range astpkg.Files {
					ast.Inspect(f, v.Visit)
				}
			}
			mutex.Lock()
			syms = append(syms, v.syms...)
			mutex.Unlock()
		}()
	})
	wg.Wait()

	sort.Slice(syms, func(i, j int) bool {
		return strings.ToLower(syms[i].Name) < strings.ToLower(syms[j].Name)
	})

	b, _ := json.MarshalIndent(syms, "", " ")
	fmt.Println(string(b))

	return nil
}
