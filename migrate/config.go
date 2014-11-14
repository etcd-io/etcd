package migrate

import (
	"encoding/json"
	"io/ioutil"

	"github.com/coreos/etcd/raft/raftpb"
)

type Config4 struct {
	CommitIndex uint64 `json:"commitIndex"`

	Peers []struct {
		Name             string `json:"name"`
		ConnectionString string `json:"connectionString"`
	} `json:"peers"`
}

func (c *Config4) HardState5() raftpb.HardState {
	return raftpb.HardState{
		Commit: c.CommitIndex,
		Term:   0,
		Vote:   0,
	}
}

func DecodeConfig4FromFile(cfgPath string) (*Config4, error) {
	b, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	conf := &Config4{}
	if err = json.Unmarshal(b, conf); err != nil {
		return nil, err
	}

	return conf, nil
}
