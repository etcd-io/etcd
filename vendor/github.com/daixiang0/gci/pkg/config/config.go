package config

import (
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/daixiang0/gci/pkg/section"
)

var defaultOrder = map[string]int{
	section.StandardType:    0,
	section.DefaultType:     1,
	section.CustomType:      2,
	section.BlankType:       3,
	section.DotType:         4,
	section.AliasType:       5,
	section.LocalModuleType: 6,
}

type BoolConfig struct {
	NoInlineComments bool `yaml:"no-inlineComments"`
	NoPrefixComments bool `yaml:"no-prefixComments"`
	Debug            bool `yaml:"-"`
	SkipGenerated    bool `yaml:"skipGenerated"`
	SkipVendor       bool `yaml:"skipVendor"`
	CustomOrder      bool `yaml:"customOrder"`
	NoLexOrder       bool `yaml:"noLexOrder"`
}

type Config struct {
	BoolConfig
	Sections          section.SectionList
	SectionSeparators section.SectionList
}

type YamlConfig struct {
	Cfg                     BoolConfig `yaml:",inline"`
	SectionStrings          []string   `yaml:"sections"`
	SectionSeparatorStrings []string   `yaml:"sectionseparators"`

	// Since history issue, Golangci-lint needs Analyzer to run and GCI add an Analyzer layer to integrate.
	// The ModPath param is only from analyzer.go, no need to set it in all other places.
	ModPath string `yaml:"-"`
}

func (g YamlConfig) Parse() (*Config, error) {
	var err error

	sections, err := section.Parse(g.SectionStrings)
	if err != nil {
		return nil, err
	}
	if sections == nil {
		sections = section.DefaultSections()
	}
	if err := configureSections(sections, g.ModPath); err != nil {
		return nil, err
	}

	// if default order sorted sections
	if !g.Cfg.CustomOrder {
		sort.Slice(sections, func(i, j int) bool {
			sectionI, sectionJ := sections[i].Type(), sections[j].Type()

			if g.Cfg.NoLexOrder || sectionI != sectionJ {
				return defaultOrder[sectionI] < defaultOrder[sectionJ]
			}

			return sections[i].String() < sections[j].String()
		})
	}

	sectionSeparators, err := section.Parse(g.SectionSeparatorStrings)
	if err != nil {
		return nil, err
	}
	if sectionSeparators == nil {
		sectionSeparators = section.DefaultSectionSeparators()
	}

	return &Config{g.Cfg, sections, sectionSeparators}, nil
}

func ParseConfig(in string) (*Config, error) {
	config := YamlConfig{}

	err := yaml.Unmarshal([]byte(in), &config)
	if err != nil {
		return nil, err
	}

	gciCfg, err := config.Parse()
	if err != nil {
		return nil, err
	}

	return gciCfg, nil
}

// configureSections now only do golang module path finding.
// Since history issue, Golangci-lint needs Analyzer to run and GCI add an Analyzer layer to integrate.
// The path param is from analyzer.go, in all other places should pass empty string.
func configureSections(sections section.SectionList, path string) error {
	for _, sec := range sections {
		switch s := sec.(type) {
		case *section.LocalModule:
			if err := s.Configure(path); err != nil {
				return err
			}
		}
	}
	return nil
}
