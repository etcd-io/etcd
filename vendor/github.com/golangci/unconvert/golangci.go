package unconvert

import (
	"go/ast"
	"go/token"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

// Transformed version of the original unconvert flags section.
// The section has been removed inside `unconvert.go`
var (
	flagAll        = pointer(false)
	flagApply      = pointer(false)
	flagCPUProfile = pointer("")
	flagSafe       = pointer(false)
	flagV          = pointer(false)
	flagTests      = pointer(true)
	flagFastMath   = pointer(false)
	flagTags       = pointer("")
	flagConfigs    = pointer("")
)

func pointer[T string | int | int32 | int64 | bool](v T) *T { return &v }

func SetFastMath(fastMath bool) {
	// To avoid race condition, the settings should not be defined during the Run.
	flagFastMath = pointer(fastMath)
}

func SetSafe(safe bool) {
	// To avoid race condition, the settings should not be defined during the Run.
	flagSafe = pointer(safe)
}

func Run(pass *analysis.Pass) []token.Position {
	type res struct {
		file  string
		edits editSet
	}

	ch := make(chan res)
	var wg sync.WaitGroup
	for _, file := range pass.Files {
		file := file

		tokenFile := pass.Fset.File(file.Package)
		filename := tokenFile.Position(file.Package).Filename

		// Hack to recognize _cgo_gotypes.go.
		if strings.HasSuffix(filename, "-d") || strings.HasSuffix(filename, "/_cgo_gotypes.go") {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			v := visitor{info: pass.TypesInfo, file: tokenFile, edits: make(editSet)}
			ast.Walk(&v, file)

			ch <- res{filename, v.edits}
		}()
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	m := make(fileToEditSet)
	for r := range ch {
		m[r.file] = r.edits
	}

	var positions []token.Position
	for _, edit := range m {
		for position, _ := range edit {
			positions = append(positions, position)
		}
	}

	return positions
}
