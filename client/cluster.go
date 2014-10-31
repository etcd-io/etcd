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

package client

import (
	"net/http"
	"net/url"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
)

func newHTTPClusterClient(tr *http.Transport, eps []string) (*httpClusterClient, error) {
	c := httpClusterClient{
		endpoints: make([]*httpClient, len(eps)),
	}

	for i, ep := range eps {
		u, err := url.Parse(ep)
		if err != nil {
			return nil, err
		}

		c.endpoints[i] = &httpClient{
			transport: tr,
			endpoint:  *u,
		}
	}

	return &c, nil
}

type httpClusterClient struct {
	endpoints []*httpClient
}

func (c *httpClusterClient) do(ctx context.Context, act httpAction) (*http.Response, []byte, error) {
	//TODO(bcwaldon): introduce retry logic so all endpoints are attempted
	return c.endpoints[0].do(ctx, act)
}
