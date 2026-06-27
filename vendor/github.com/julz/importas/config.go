package importas

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
)

type Config struct {
	RequiredAlias        aliasList
	Rules                []*Rule
	DisallowUnaliased    bool
	DisallowExtraAliases bool
	muRules              sync.Mutex
}

func (c *Config) CompileRegexp() error {
	c.muRules.Lock()
	defer c.muRules.Unlock()
	if c.Rules != nil {
		return nil
	}
	rules := make([]*Rule, 0, len(c.RequiredAlias))
	for _, aliases := range c.RequiredAlias {
		path, alias := aliases[0], aliases[1]
		reg, err := regexp.Compile(fmt.Sprintf("^%s$", path))
		if err != nil {
			return err
		}

		rules = append(rules, &Rule{
			Regexp: reg,
			Alias:  alias,
		})
	}
	c.Rules = rules
	return nil
}

func (c *Config) findRule(path string) *Rule {
	c.muRules.Lock()
	rules := c.Rules
	c.muRules.Unlock()
	for _, rule := range rules {
		if rule.Regexp.MatchString(path) {
			return rule
		}
	}

	return nil
}

func (c *Config) AliasFor(path string) (string, bool) {
	rule := c.findRule(path)
	if rule == nil {
		return "", false
	}

	alias, err := rule.aliasFor(path)
	if err != nil {
		return "", false
	}

	return alias, true
}

type Rule struct {
	Alias  string
	Regexp *regexp.Regexp
}

func (r *Rule) aliasFor(path string) (string, error) {
	str := r.Regexp.FindString(path)
	if len(str) > 0 {
		return r.Regexp.ReplaceAllString(str, r.Alias), nil
	}

	return "", errors.New("mismatch rule")
}
