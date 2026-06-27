package section

import (
	"github.com/daixiang0/gci/pkg/parse"
	"github.com/daixiang0/gci/pkg/specificity"
)

const newLineName = "newline"

type NewLine struct{}

func (n NewLine) MatchSpecificity(spec *parse.GciImports) specificity.MatchSpecificity {
	return specificity.MisMatch{}
}

func (n NewLine) String() string {
	return newLineName
}

func (n NewLine) Type() string {
	return newLineName
}
