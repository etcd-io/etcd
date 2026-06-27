package section

import "github.com/daixiang0/gci/pkg/section"

func DefaultSections() section.SectionList {
	return section.SectionList{Standard{}, section.Default{}}
}
