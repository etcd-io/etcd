/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdserver

import (
	"testing"
)

func TestConfigVerify(t *testing.T) {
	tests := []struct {
		clusterSetting string
		shouldError    bool
	}{
		{"", true},
		{"node1=http://localhost:7001,node2=http://localhost:7001", true},
		{"node1=http://localhost:7001,node2=http://localhost:7002", false},
	}

	for i, tt := range tests {
		cluster := &Cluster{}
		cluster.Set(tt.clusterSetting)
		cfg := ServerConfig{
			Name:    "node1",
			Cluster: cluster,
		}
		err := cfg.Verify()
		if (err == nil) && tt.shouldError {
			t.Errorf("#%d: Got no error where one was expected", i)
		}
		if (err != nil) && !tt.shouldError {
			t.Errorf("#%d: Got unexpected error: %v", i, err)
		}
	}
}
