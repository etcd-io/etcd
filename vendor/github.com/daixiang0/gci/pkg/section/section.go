package section

import (
	"github.com/daixiang0/gci/pkg/parse"
	"github.com/daixiang0/gci/pkg/specificity"
)

// Section defines a part of the formatted output.
type Section interface {
	// MatchSpecificity returns how well an Import matches to this Section
	MatchSpecificity(spec *parse.GciImports) specificity.MatchSpecificity

	// String Implements the stringer interface
	String() string

	// return section type
	Type() string
}

type SectionList []Section

func (list SectionList) String() []string {
	var output []string
	for _, section := range list {
		output = append(output, section.String())
	}
	return output
}

func DefaultSections() SectionList {
	return SectionList{Standard{}, Default{}}
}

func DefaultSectionSeparators() SectionList {
	return SectionList{NewLine{}}
}
