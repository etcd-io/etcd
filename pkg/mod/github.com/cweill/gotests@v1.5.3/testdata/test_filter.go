package testdata

import "errors"

func FooFilter(strs []string) ([]*Bar, error) { return nil, nil }

func (b *Bar) BarFilter(i interface{}) error {
	if i == nil {
		return errors.New("i is nil")
	}
	return nil
}

func bazFilter(f *float64) float64 { return *f }
