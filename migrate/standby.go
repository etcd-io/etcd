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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type StandbyInfo4 struct {
	Running      bool
	Cluster      []*MachineMessage
	SyncInterval float64
}

// MachineMessage represents information about a peer or standby in the registry.
type MachineMessage struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	ClientURL string `json:"clientURL"`
	PeerURL   string `json:"peerURL"`
}

func (si *StandbyInfo4) ClientURLs() []string {
	var urls []string
	for _, m := range si.Cluster {
		urls = append(urls, m.ClientURL)
	}
	return urls
}

func (si *StandbyInfo4) InitialCluster() string {
	b := &bytes.Buffer{}
	first := true
	for _, m := range si.Cluster {
		if !first {
			fmt.Fprintf(b, ",")
		}
		first = false
		fmt.Fprintf(b, "%s=%s", m.Name, m.PeerURL)
	}
	return b.String()
}

func DecodeStandbyInfo4FromFile(path string) (*StandbyInfo4, error) {
	var info StandbyInfo4
	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err = json.NewDecoder(file).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
