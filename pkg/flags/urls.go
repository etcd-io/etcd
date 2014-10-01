package flags

import (
	"strings"

	"github.com/coreos/etcd/pkg/types"
)

type URLsValue types.URLs

// Set parses a command line set of URLs formatted like:
// http://127.0.0.1:7001,http://10.1.1.2:80
func (us *URLsValue) Set(s string) error {
	strs := strings.Split(s, ",")
	nus, err := types.NewURLs(strs)
	if err != nil {
		return err
	}

	*us = URLsValue(nus)
	return nil
}

func (us *URLsValue) String() string {
	all := make([]string, len(*us))
	for i, u := range *us {
		all[i] = u.String()
	}
	return strings.Join(all, ",")
}

func NewURLsValue(init string) *URLsValue {
	v := &URLsValue{}
	v.Set(init)
	return v
}
