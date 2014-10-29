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
	"testing"
)

func TestMembersAPIListAction(t *testing.T) {
	ep := url.URL{Scheme: "http", Host: "example.com/v2/members"}
	wantURL := &url.URL{
		Scheme: "http",
		Host:   "example.com",
		Path:   "/v2/members",
	}

	act := &membersAPIActionList{}
	got := *act.httpRequest(ep)
	err := assertResponse(got, wantURL, http.Header{}, nil)
	if err != nil {
		t.Errorf(err.Error())
	}
}
