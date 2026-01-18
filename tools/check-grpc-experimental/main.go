// Copyright 2026 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

var (
	debugMode     = flag.Bool("debug", false, "enable verbose debug logging")
	allowListFile = flag.String("allow-list", "", "path to a file containing allowed APIs (one per line)")
)

// Map to store allowed signatures (e.g., "grpc.WithResolvers" -> true)
var allowList = make(map[string]bool)

func main() {
	flag.Parse()
	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	if *allowListFile != "" {
		if err := loadAllowList(*allowListFile); err != nil {
			log.Fatalf("Failed to load allow list: %v", err)
		}
	}

	// Load source with type info.
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Tests: true,
	}

	if *debugMode {
		log.Println("Loading packages...")
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		log.Fatalf("failed to load packages: %v", err)
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		os.Exit(1)
	}

	if *debugMode {
		log.Printf("Loaded %d packages. Scanning for gRPC usage...", len(pkgs))
	}

	foundExperimental := false

	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}

				obj := pkg.TypesInfo.Uses[sel.Sel]
				if obj == nil || obj.Pkg() == nil {
					return true
				}

				// Strict filter for gRPC
				if !strings.Contains(obj.Pkg().Path(), "google.golang.org/grpc") {
					return true
				}

				// Check Allowlist
				// Construct the signature: PackageName.Symbol (e.g. "grpc.WithResolvers", "resolver.Address")
				signature := obj.Pkg().Name() + "." + obj.Name()
				if allowList[signature] {
					if *debugMode {
						log.Printf("Ignoring allowed usage: %s", signature)
					}
					return true
				}

				if *debugMode {
					log.Printf("Checking reference: %s", signature)
				}

				if isExperimental(pkg.Fset, obj) {
					pos := pkg.Fset.Position(sel.Pos())
					fmt.Printf("%s:%d:%d: usage of experimental gRPC API: %s\n",
						pos.Filename, pos.Line, pos.Column, signature)
					foundExperimental = true
				}

				return true
			})
		}
	}

	if foundExperimental {
		os.Exit(1)
	}
}

var (
	experimentalRegex = regexp.MustCompile(`(?i)(#\s*Experimental|All APIs in this package are experimental|This API is EXPERIMENTAL|is currently experimental)`)
)

func loadAllowList(fpath string) error {
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowList[line] = true
	}
	return scanner.Err()
}

func isExperimental(mainFset *token.FileSet, obj types.Object) bool {
	pos := obj.Pos()
	if !pos.IsValid() {
		return false
	}

	// Get the absolute path to the dependency file
	position := mainFset.Position(pos)
	filename := position.Filename

	// Skip if it's not a Go file (e.g. built-in types might have no file)
	if !strings.HasSuffix(filename, ".go") {
		return false
	}

	if *debugMode {
		// Log checking definition
		log.Printf("  -> Definition found at: %s:%d", filename, position.Line)
	}

	return checkFileForExperimental(filename, position.Line)
}

// Cached file structure
type cachedFile struct {
	f       *ast.File
	fset    *token.FileSet
	src     []byte
	hasDocs bool
}

var (
	cacheMu       sync.RWMutex
	advancedCache = make(map[string]*cachedFile)
)

func getParsedFile(filename string) (*cachedFile, error) {
	cacheMu.RLock()
	if v, ok := advancedCache[filename]; ok {
		cacheMu.RUnlock()
		return v, nil
	}
	cacheMu.RUnlock()

	// Parse the file
	fset := token.NewFileSet()
	// We read the file content manually to help with debugging if needed
	src, err := os.ReadFile(filename)
	if err != nil {
		if *debugMode {
			log.Printf("ERROR reading file %s: %v", filename, err)
		}
		return nil, err
	}

	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}

	res := &cachedFile{f: f, fset: fset, src: src, hasDocs: f.Doc != nil}

	cacheMu.Lock()
	advancedCache[filename] = res
	cacheMu.Unlock()

	return res, nil
}

func checkFileForExperimental(filename string, targetLine int) bool {
	cf, err := getParsedFile(filename)
	if err != nil {
		return false
	}

	// check package-level comments
	if cf.f.Doc != nil {
		if experimentalRegex.MatchString(cf.f.Doc.Text()) {
			if *debugMode {
				log.Printf("  -> [MATCH] Package experimental (doc in file): %s", filename)
			}
			return true
		}
	}

	// check package-level comment in doc.go
	// If the current file didn't have the experimental tag, check if a doc.go exists in the same folder
	dir := filepath.Dir(filename)
	docPath := filepath.Join(dir, "doc.go")
	// Only check doc.go if we aren't already looking at it
	if docPath != filename {
		if docContent, err := os.ReadFile(docPath); err == nil {
			if experimentalRegex.Match(docContent) {
				if *debugMode {
					log.Printf("  -> [MATCH] Package experimental (found in doc.go): %s", docPath)
				}
				return true
			}
		}
	}

	// check specific object comments
	found := false

	ast.Inspect(cf.f, func(n ast.Node) bool {
		if found {
			return false
		}
		if n == nil {
			return true
		}

		// Helper to check a comment group
		checkDoc := func(doc *ast.CommentGroup, name string) {
			if doc != nil && experimentalRegex.MatchString(doc.Text()) {
				found = true
				if *debugMode {
					log.Printf("  -> [MATCH] Object experimental: %s", name)
				}
			}
		}

		switch decl := n.(type) {
		case *ast.FuncDecl:
			// Match if the target line is within the function declaration lines
			// Actually, we want the definition line exactly, or close to it.
			start := cf.fset.Position(decl.Pos()).Line
			// The object.Pos() points to the name, not the 'func' keyword, usually.
			namePos := cf.fset.Position(decl.Name.Pos()).Line

			if namePos == targetLine || start == targetLine {
				checkDoc(decl.Doc, decl.Name.Name)
			}

		case *ast.GenDecl:
			// GenDecl covers `type X struct`, `var X`, `const X`
			// The GenDecl doc applies to all specs inside it usually.

			// If the GenDecl itself starts on the line (e.g. `type ( ...`)
			// or if it contains our line.
			start := cf.fset.Position(decl.Pos()).Line
			end := cf.fset.Position(decl.End()).Line

			if targetLine >= start && targetLine <= end {
				// Check the top-level GenDecl doc (e.g. "// Experimental\n var ( ... )")
				if decl.Doc != nil && experimentalRegex.MatchString(decl.Doc.Text()) {
					found = true
					if *debugMode {
						log.Printf("  -> [MATCH] GenDecl experimental block around line %d", targetLine)
					}
					return false
				}

				// Check individual specs
				for _, spec := range decl.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if cf.fset.Position(s.Name.Pos()).Line == targetLine {
							checkDoc(s.Doc, s.Name.Name)
						}
					case *ast.ValueSpec: // var/const
						for _, name := range s.Names {
							if cf.fset.Position(name.Pos()).Line == targetLine {
								checkDoc(s.Doc, name.Name)
							}
						}
					}
				}
			}
		}
		return true
	})

	return found
}
