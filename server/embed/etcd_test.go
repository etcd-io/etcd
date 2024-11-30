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

package embed

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/etcd/client/pkg/v3/transport"
)

func TestEmptyClientTLSInfo_createMetricsListener(t *testing.T) {
	e := &Etcd{
		cfg: Config{
			ClientTLSInfo: transport.TLSInfo{},
		},
	}

	murl := url.URL{
		Scheme: "https",
		Host:   "localhost:8080",
	}
	_, err := e.createMetricsListener(murl)
	require.ErrorIsf(t, err, ErrMissingClientTLSInfoForMetricsURL, "expected error %v, got %v", ErrMissingClientTLSInfoForMetricsURL, err)
}
