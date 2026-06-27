package checkcompilerdirectives

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

func Analyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "gocheckcompilerdirectives",
		Doc:  "Checks that go compiler directive comments (//go:) are valid.",
		Run:  run,
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				text := comment.Text
				if !strings.HasPrefix(text, "//") {
					continue
				}
				start := 2
				spaces := 0
				for _, c := range text[start:] {
					if c == ' ' {
						spaces++
						continue
					}
					break
				}
				start += spaces
				if !strings.HasPrefix(text[start:], "go:") {
					continue
				}
				start += 3
				end := strings.Index(text[start:], " ")
				if end == -1 {
					continue
				}
				directive := text[start : start+end]
				if len(directive) == 0 {
					continue
				}
				prefix := text[:start+end]
				// Leading whitespace will cause the go directive to be ignored
				// by the compiler with no error, causing it not to work. This
				// is an easy mistake.
				if spaces > 0 {
					pass.ReportRangef(comment, "compiler directive contains space: %s", prefix)
				}
				// If the directive is unknown it will be ignored by the
				// compiler with no error. This is an easy mistake to make,
				// especially if you typo a directive.
				if !isKnown(directive) {
					pass.ReportRangef(comment, "compiler directive unrecognized: %s", prefix)
				}
			}
		}
	}
	return nil, nil
}

func isKnown(directive string) bool {
	for _, k := range known {
		if directive == k {
			return true
		}
	}
	return false
}

// Found by running the following command on the source of go.
// git grep -o -E -h '//go:[a-z_-]+' -- ':!**/*_test.go' ':!test/' ':!**/testdata/**' | sort -u
// See https://pkg.go.dev/cmd/compile@go1.24#hdr-Compiler_Directives
var known = []string{
	"build",
	"cgo_dynamic_linker",
	"cgo_export_dynamic",
	"cgo_export_static",
	"cgo_import_dynamic",
	"cgo_import_static",
	"cgo_ldflag",
	"cgo_unsafe_args",
	"debug",
	"embed",
	"generate",
	"linkname",
	"nocheckptr",
	"noescape",
	"noinline",
	"nointerface",
	"norace",
	"nosplit",
	"notinheap",
	"nowritebarrier",
	"nowritebarrierrec",
	"systemstack",
	"uintptrescapes",
	"uintptrkeepalive",
	"wasmimport",
	"wasmexport",
	"yeswritebarrierrec",
}
