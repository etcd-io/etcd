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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"time"
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
	List() ([]Member, error)
}

type Member struct {
	ID         uint64
	Name       string
	PeerURLs   []url.URL
	ClientURLs []url.URL
}

func (m *Member) UnmarshalJSON(data []byte) (err error) {
	rm := struct {
		ID         uint64
		Name       string
		PeerURLs   []string
		ClientURLs []string
	}{}

	if err := json.Unmarshal(data, &rm); err != nil {
		return err
	}

	parseURLs := func(strs []string) ([]url.URL, error) {
		urls := make([]url.URL, len(strs))
		for i, s := range strs {
			u, err := url.Parse(s)
			if err != nil {
				return nil, err
			}
			urls[i] = *u
		}

		return urls, nil
	}

	if m.PeerURLs, err = parseURLs(rm.PeerURLs); err != nil {
		return err
	}

	if m.ClientURLs, err = parseURLs(rm.ClientURLs); err != nil {
		return err
	}

	m.ID = rm.ID
	m.Name = rm.Name

	return nil
}

type membersCollection struct {
	Members []Member
}

type httpMembersAPI struct {
	client *httpClient
}

func (m *httpMembersAPI) List() ([]Member, error) {
	httpresp, body, err := m.client.doWithTimeout(&membersAPIActionList{})
	if err != nil {
		return nil, err
	}

	mResponse := httpMembersAPIResponse{
		code: httpresp.StatusCode,
		body: body,
	}

	if err = mResponse.err(); err != nil {
		return nil, err
	}

	var mCollection membersCollection
	if err = mResponse.unmarshalBody(&mCollection); err != nil {
		return nil, err
	}

	return mCollection.Members, nil
}

type httpMembersAPIResponse struct {
	code int
	body []byte
}

func (r *httpMembersAPIResponse) err() (err error) {
	if r.code != http.StatusOK {
		err = fmt.Errorf("unrecognized status code %d", r.code)
	}
	return
}

func (r *httpMembersAPIResponse) unmarshalBody(dst interface{}) (err error) {
	return json.Unmarshal(r.body, dst)
}

type membersAPIActionList struct{}

func (l *membersAPIActionList) httpRequest(ep url.URL) *http.Request {
	req, _ := http.NewRequest("GET", ep.String(), nil)
	return req
}
