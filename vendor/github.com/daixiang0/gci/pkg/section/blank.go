package section

import (
	"github.com/daixiang0/gci/pkg/parse"
	"github.com/daixiang0/gci/pkg/specificity"
)

type Blank struct{}

const BlankType = "blank"

func (b Blank) MatchSpecificity(spec *parse.GciImports) specificity.MatchSpecificity {
	if spec.Name == "_" {
		return specificity.NameMatch{}
	}
	return specificity.MisMatch{}
}

func (b Blank) String() string {
	return BlankType
}

func (b Blank) Type() string {
	return BlankType
}
