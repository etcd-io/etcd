// Copyright 2024 The etcd Authors
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

package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestNoErrorLogsDuringNormalOperations(t *testing.T) {
	e2e.BeforeTest(t)
	ctx := context.TODO()

	epc, err := e2e.NewEtcdProcessCluster(ctx, t,
		e2e.WithClusterSize(1),
		e2e.WithLogLevel("debug"),
	)
	require.NoError(t, err)
	defer epc.Close()

	logs := epc.Procs[0].Logs()
	time.Sleep(time.Second)
	if err = epc.Close(); err != nil {
		t.Fatalf("error closing etcd processes (%v)", err)
	}
	var entry logEntry
	lines := logs.Lines()
	if len(lines) == 0 {
		t.Errorf("Expected at least one log line")
	}

	allowedErrors := map[string]bool{"setting up serving from embedded etcd failed.": true}

	for _, line := range lines {
		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Errorf("Failed to parse log line as json, err: %q, line: %s", err, line)
			continue
		}
		if allowedErrors[entry.Message] {
			continue
		}

		require.NotEqual(t, "error", entry.Level)
	}
}
