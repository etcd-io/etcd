package section

import (
	"github.com/daixiang0/gci/pkg/parse"
	"github.com/daixiang0/gci/pkg/specificity"
)

type Alias struct{}

const AliasType = "alias"

func (b Alias) MatchSpecificity(spec *parse.GciImports) specificity.MatchSpecificity {
	if spec.Name != "." && spec.Name != "_" && spec.Name != "" {
		return specificity.NameMatch{}
	}
	return specificity.MisMatch{}
}

func (b Alias) String() string {
	return AliasType
}

func (b Alias) Type() string {
	return AliasType
}
