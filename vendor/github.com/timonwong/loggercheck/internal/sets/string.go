package sets

import (
	"sort"
	"strings"
)

type Empty struct{}

type StringSet map[string]Empty

func NewString(items ...string) StringSet {
	s := make(StringSet)
	s.Insert(items...)
	return s
}

func (s StringSet) Insert(items ...string) {
	for _, item := range items {
		s[item] = Empty{}
	}
}

func (s StringSet) Has(item string) bool {
	_, contained := s[item]
	return contained
}

func (s StringSet) List() []string {
	if len(s) == 0 {
		return nil
	}

	res := make([]string, 0, len(s))
	for key := range s {
		res = append(res, key)
	}
	sort.Strings(res)
	return res
}

// Set implements flag.Value interface.
func (s *StringSet) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		*s = nil
		return nil
	}

	parts := strings.Split(v, ",")
	set := NewString(parts...)
	*s = set
	return nil
}

// String implements flag.Value interface
func (s StringSet) String() string {
	return strings.Join(s.List(), ",")
}
