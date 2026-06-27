package processors

import (
	"path/filepath"
	"strings"

	"github.com/ldez/grignotin/goenv"

	"github.com/golangci/golangci-lint/v2/pkg/goutil"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*Cgo)(nil)

// Cgo filters cgo artifacts.
//
// Some linters (e.g. gosec, etc.) return incorrect file paths for cgo files.
//
// Require absolute file path.
type Cgo struct {
	goCacheDir string
}

func NewCgo(env *goutil.Env) *Cgo {
	return &Cgo{
		goCacheDir: env.Get(goenv.GOCACHE),
	}
}

func (*Cgo) Name() string {
	return "cgo"
}

func (p *Cgo) Process(issues []*result.Issue) ([]*result.Issue, error) {
	return filterIssuesErr(issues, p.shouldPassIssue)
}

func (*Cgo) Finish() {}

func (p *Cgo) shouldPassIssue(issue *result.Issue) (bool, error) {
	// [p.goCacheDir] contains all preprocessed files including cgo files.
	if p.goCacheDir != "" && strings.HasPrefix(issue.FilePath(), p.goCacheDir) {
		return false, nil
	}

	if filepath.Base(issue.FilePath()) == "_cgo_gotypes.go" {
		// skip cgo warning for go1.10
		return false, nil
	}

	return true, nil
}
