package etcdserver

import (
	"errors"
)

const (
	ClusterStateValueNew = "new"
)

var (
	ClusterStateValues = []string{
		ClusterStateValueNew,
	}
)

// ClusterState implements the flag.Value interface.
type ClusterState string

// Set verifies the argument to be a valid member of ClusterStateFlagValues
// before setting the underlying flag value.
func (cs *ClusterState) Set(s string) error {
	for _, v := range ClusterStateValues {
		if s == v {
			*cs = ClusterState(s)
			return nil
		}
	}

	return errors.New("invalid value")
}

func (cs *ClusterState) String() string {
	return string(*cs)
}
