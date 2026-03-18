// Copyright 2017 The etcd Authors
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

package embed

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/server/v3/auth"
	"go.etcd.io/etcd/server/v3/embed/testutil"
)

// TestStartEtcdWrongToken ensures that StartEtcd with wrong configs returns with error.
func TestStartEtcdWrongToken(t *testing.T) {
	tdir := t.TempDir()

	cfg := NewConfig()

	testURLConfg := testutil.NewConfigTestURLs()
	cfg.ListenClientUrls, cfg.AdvertiseClientUrls = testURLConfg.ClientURLs, testURLConfg.ClientURLs
	cfg.ListenPeerUrls, cfg.AdvertisePeerUrls = testURLConfg.PeerURLs, testURLConfg.PeerURLs
	cfg.InitialCluster = testURLConfg.InitialCluster

	cfg.Dir = tdir
	cfg.AuthToken = "wrong-token"

	_, err := StartEtcd(cfg)
	require.ErrorIsf(t, err, auth.ErrInvalidAuthOpts, "expected %v, got %v", auth.ErrInvalidAuthOpts, err)
}
