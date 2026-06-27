package printer

// Golangci-lint: altered version of plumbing.go

import (
	"sort"

	"github.com/golangci/dupl/syntax"
)

type Clone clone

func (c Clone) Filename() string {
	return c.filename
}

func (c Clone) LineStart() int {
	return c.lineStart
}

func (c Clone) LineEnd() int {
	return c.lineEnd
}

type Issue struct {
	From, To Clone
}

type Issuer struct {
	ReadFile
}

func NewIssuer(fread ReadFile) *Issuer {
	return &Issuer{fread}
}

func (p *Issuer) MakeIssues(dups [][]*syntax.Node) ([]Issue, error) {
	clones, err := prepareClonesInfo(p.ReadFile, dups)
	if err != nil {
		return nil, err
	}

	sort.Sort(byNameAndLine(clones))

	var issues []Issue

	for i, cl := range clones {
		nextCl := clones[(i+1)%len(clones)]
		issues = append(issues, Issue{
			From: Clone(cl),
			To:   Clone(nextCl),
		})
	}

	return issues, nil
}
