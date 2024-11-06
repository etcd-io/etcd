// Copyright 2024 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestDefragNoSpace(t *testing.T) {
	tests := []struct {
		name      string
		failpoint string
		err       string
	}{
		{
			name:      "no space (#18810) - can't open/create new bbolt db",
			failpoint: "defragOpenFileError",
			err:       "no space",
		},
		{
			name:      "defragdb failure",
			failpoint: "defragdbFail",
			err:       "some random error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e2e.BeforeTest(t)

			clus, err := e2e.NewEtcdProcessCluster(context.TODO(), t,
				e2e.WithClusterSize(1),
				e2e.WithGoFailEnabled(true),
			)
			require.NoError(t, err)
			t.Cleanup(func() { clus.Stop() })

			member := clus.Procs[0]

			require.NoError(t, member.Failpoints().SetupHTTP(context.Background(), tc.failpoint, fmt.Sprintf(`return("%s")`, tc.err)))
			require.ErrorContains(t, member.Etcdctl().Defragment(context.Background(), config.DefragOption{Timeout: time.Minute}), tc.err)

			// Make sure etcd continues to run even after the failed defrag attempt
			require.NoError(t, member.Etcdctl().Put(context.Background(), "foo", "bar", config.PutOptions{}))
			value, err := member.Etcdctl().Get(context.Background(), "foo", config.GetOptions{})
			require.NoError(t, err)
			require.Len(t, value.Kvs, 1)
			require.Equal(t, "bar", string(value.Kvs[0].Value))
		})
	}
}
