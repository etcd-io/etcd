// Copyright 2015 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func (c *Config4) HardState2() raftpb.HardState {
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
