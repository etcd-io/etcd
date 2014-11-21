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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
	"github.com/coreos/etcd/pkg/types"
)

var (
	DefaultV2MembersPrefix = "/v2/members"
)

func NewMembersAPI(c HTTPClient) MembersAPI {
	return &httpMembersAPI{
		client: c,
	}
}

type MembersAPI interface {
	List(ctx context.Context) ([]httptypes.Member, error)
	Add(ctx context.Context, peerURL string) (*httptypes.Member, error)
	Remove(ctx context.Context, mID string) error
}

type httpMembersAPI struct {
	client HTTPClient
}

func (m *httpMembersAPI) List(ctx context.Context) ([]httptypes.Member, error) {
	req := &membersAPIActionList{}
	resp, body, err := m.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(resp.StatusCode, http.StatusOK); err != nil {
		return nil, err
	}

	var mCollection httptypes.MemberCollection
	if err := json.Unmarshal(body, &mCollection); err != nil {
		return nil, err
	}

	return []httptypes.Member(mCollection), nil
}

func (m *httpMembersAPI) Add(ctx context.Context, peerURL string) (*httptypes.Member, error) {
	urls, err := types.NewURLs([]string{peerURL})
	if err != nil {
		return nil, err
	}

	req := &membersAPIActionAdd{peerURLs: urls}
	resp, body, err := m.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(resp.StatusCode, http.StatusCreated, http.StatusConflict); err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		var httperr httptypes.HTTPError
		if err := json.Unmarshal(body, &httperr); err != nil {
			return nil, err
		}
		return nil, httperr
	}

	var memb httptypes.Member
	if err := json.Unmarshal(body, &memb); err != nil {
		return nil, err
	}

	return &memb, nil
}

func (m *httpMembersAPI) Remove(ctx context.Context, memberID string) error {
	req := &membersAPIActionRemove{memberID: memberID}
	resp, _, err := m.client.Do(ctx, req)
	if err != nil {
		return err
	}

	return assertStatusCode(resp.StatusCode, http.StatusNoContent)
}

type membersAPIActionList struct{}

func (l *membersAPIActionList) HTTPRequest(ep url.URL) *http.Request {
	u := v2MembersURL(ep)
	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

type membersAPIActionRemove struct {
	memberID string
}

func (d *membersAPIActionRemove) HTTPRequest(ep url.URL) *http.Request {
	u := v2MembersURL(ep)
	u.Path = path.Join(u.Path, d.memberID)
	req, _ := http.NewRequest("DELETE", u.String(), nil)
	return req
}

type membersAPIActionAdd struct {
	peerURLs types.URLs
}

func (a *membersAPIActionAdd) HTTPRequest(ep url.URL) *http.Request {
	u := v2MembersURL(ep)
	m := httptypes.MemberCreateRequest{PeerURLs: a.peerURLs}
	b, _ := json.Marshal(&m)
	req, _ := http.NewRequest("POST", u.String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertStatusCode(got int, want ...int) (err error) {
	for _, w := range want {
		if w == got {
			return nil
		}
	}
	return fmt.Errorf("unexpected status code %d", got)
}

// v2MembersURL add the necessary path to the provided endpoint
// to route requests to the default v2 members API.
func v2MembersURL(ep url.URL) *url.URL {
	ep.Path = path.Join(ep.Path, DefaultV2MembersPrefix)
	return &ep
}
