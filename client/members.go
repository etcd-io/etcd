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
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
	"github.com/coreos/etcd/pkg/types"
)

var (
	DefaultV2MembersPrefix = "/v2/members"
)

func NewMembersAPI(tr *http.Transport, eps []string, to time.Duration) (MembersAPI, error) {
	c, err := NewHTTPClient(tr, eps)
	if err != nil {
		return nil, err
	}

	mAPI := httpMembersAPI{
		client:  c,
		timeout: to,
	}

	return &mAPI, nil
}

type MembersAPI interface {
	List() ([]httptypes.Member, error)
	Add(peerURL string) (*httptypes.Member, error)
	Remove(mID string) error
}

type httpMembersAPI struct {
	client  httpActionDo
	timeout time.Duration
}

func (m *httpMembersAPI) List() ([]httptypes.Member, error) {
	req := &membersAPIActionList{}
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	resp, body, err := m.client.do(ctx, req)
	cancel()
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(http.StatusOK, resp.StatusCode); err != nil {
		return nil, err
	}

	var mCollection httptypes.MemberCollection
	if err := json.Unmarshal(body, &mCollection); err != nil {
		return nil, err
	}

	return []httptypes.Member(mCollection), nil
}

func (m *httpMembersAPI) Add(peerURL string) (*httptypes.Member, error) {
	urls, err := types.NewURLs([]string{peerURL})
	if err != nil {
		return nil, err
	}

	req := &membersAPIActionAdd{peerURLs: urls}
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	resp, body, err := m.client.do(ctx, req)
	cancel()
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(http.StatusCreated, resp.StatusCode); err != nil {
		return nil, err
	}

	var memb httptypes.Member
	if err := json.Unmarshal(body, &memb); err != nil {
		return nil, err
	}

	return &memb, nil
}

func (m *httpMembersAPI) Remove(memberID string) error {
	req := &membersAPIActionRemove{memberID: memberID}
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	resp, _, err := m.client.do(ctx, req)
	cancel()
	if err != nil {
		return err
	}

	return assertStatusCode(http.StatusNoContent, resp.StatusCode)
}

type membersAPIActionList struct{}

func (l *membersAPIActionList) httpRequest(ep url.URL) *http.Request {
	u := v2MembersURL(ep)
	req, _ := http.NewRequest("GET", u.String(), nil)
	return req
}

type membersAPIActionRemove struct {
	memberID string
}

func (d *membersAPIActionRemove) httpRequest(ep url.URL) *http.Request {
	u := v2MembersURL(ep)
	u.Path = path.Join(u.Path, d.memberID)
	req, _ := http.NewRequest("DELETE", u.String(), nil)
	return req
}

type membersAPIActionAdd struct {
	peerURLs types.URLs
}

func (a *membersAPIActionAdd) httpRequest(ep url.URL) *http.Request {
	u := v2MembersURL(ep)
	m := httptypes.MemberCreateRequest{PeerURLs: a.peerURLs}
	b, _ := json.Marshal(&m)
	req, _ := http.NewRequest("POST", u.String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertStatusCode(want, got int) (err error) {
	if want != got {
		err = fmt.Errorf("unexpected status code %d", got)
	}
	return err
}

// v2MembersURL add the necessary path to the provided endpoint
// to route requests to the default v2 members API.
func v2MembersURL(ep url.URL) *url.URL {
	ep.Path = path.Join(ep.Path, DefaultV2MembersPrefix)
	return &ep
}
