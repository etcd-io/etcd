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

	"github.com/coreos/etcd/etcdserver/etcdhttp/httptypes"
)

var (
	DefaultV2MembersPrefix = "/v2/admin/members"
)

func NewMembersAPI(tr *http.Transport, ep string, to time.Duration) (MembersAPI, error) {
	u, err := url.Parse(ep)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, DefaultV2MembersPrefix)

	c := &httpClient{
		transport: tr,
		endpoint:  *u,
		timeout:   to,
	}

	mAPI := httpMembersAPI{
		client: c,
	}

	return &mAPI, nil
}

type MembersAPI interface {
	List() ([]httptypes.Member, error)
	Add(peerURL string) (*httptypes.Member, error)
	Remove(mID string) error
}

type httpMembersAPI struct {
	client *httpClient
}

func (m *httpMembersAPI) List() ([]httptypes.Member, error) {
	httpresp, body, err := m.client.doWithTimeout(&membersAPIActionList{})
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(http.StatusOK, httpresp.StatusCode); err != nil {
		return nil, err
	}

	var mCollection httptypes.MemberCollection
	if err := json.Unmarshal(body, &mCollection); err != nil {
		return nil, err
	}

	return []httptypes.Member(mCollection), nil
}

func (m *httpMembersAPI) Add(peerURL string) (*httptypes.Member, error) {
	req := &membersAPIActionAdd{peerURL: peerURL}
	httpresp, body, err := m.client.doWithTimeout(req)
	if err != nil {
		return nil, err
	}

	if err := assertStatusCode(http.StatusCreated, httpresp.StatusCode); err != nil {
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
	httpresp, _, err := m.client.doWithTimeout(req)
	if err != nil {
		return err
	}

	return assertStatusCode(http.StatusNoContent, httpresp.StatusCode)
}

type membersAPIActionList struct{}

func (l *membersAPIActionList) httpRequest(ep url.URL) *http.Request {
	req, _ := http.NewRequest("GET", ep.String(), nil)
	return req
}

type membersAPIActionRemove struct {
	memberID string
}

func (d *membersAPIActionRemove) httpRequest(ep url.URL) *http.Request {
	ep.Path = path.Join(ep.Path, d.memberID)
	req, _ := http.NewRequest("DELETE", ep.String(), nil)
	return req
}

type membersAPIActionAdd struct {
	peerURL string
}

func (a *membersAPIActionAdd) httpRequest(ep url.URL) *http.Request {
	m := httptypes.Member{PeerURLs: []string{a.peerURL}}
	b, _ := json.Marshal(&m)
	req, _ := http.NewRequest("POST", ep.String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertStatusCode(want, got int) (err error) {
	if want != got {
		err = fmt.Errorf("unexpected status code %d", got)
	}
	return err
}
