// Copyright 2015 CoreOS, Inc.
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

package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"path"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
)

var (
	defaultV2SecurityPrefix = "/v2/security"
)

type User struct {
	User     string   `json:"user"`
	Password string   `json:"password,omitempty"`
	Roles    []string `json:"roles"`
	Grant    []string `json:"grant,omitempty"`
	Revoke   []string `json:"revoke,omitempty"`
}

func v2SecurityURL(ep url.URL, action string, name string) *url.URL {
	if name != "" {
		ep.Path = path.Join(ep.Path, defaultV2SecurityPrefix, action, name)
		return &ep
	}
	ep.Path = path.Join(ep.Path, defaultV2SecurityPrefix, action)
	return &ep
}

// NewSecurityAPI constructs a new SecurityAPI that uses HTTP to
// interact with etcd's general security features.
func NewSecurityAPI(c Client) SecurityAPI {
	return &httpSecurityAPI{
		client: c,
	}
}

type SecurityAPI interface {
	// Enable security.
	Enable(ctx context.Context) error

	// Disable security.
	Disable(ctx context.Context) error
}

type httpSecurityAPI struct {
	client httpClient
}

func (s *httpSecurityAPI) Enable(ctx context.Context) error {
	return s.enableDisable(ctx, &securityAPIAction{"PUT"})
}

func (s *httpSecurityAPI) Disable(ctx context.Context) error {
	return s.enableDisable(ctx, &securityAPIAction{"DELETE"})
}

func (s *httpSecurityAPI) enableDisable(ctx context.Context, req httpAction) error {
	resp, body, err := s.client.Do(ctx, req)
	if err != nil {
		return err
	}
	if err := assertStatusCode(resp.StatusCode, http.StatusOK, http.StatusCreated); err != nil {
		var sec securityError
		err := json.Unmarshal(body, &sec)
		if err != nil {
			return err
		}
		return sec
	}
	return nil
}

type securityAPIAction struct {
	verb string
}

func (l *securityAPIAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2SecurityURL(ep, "enable", "")
	req, _ := http.NewRequest(l.verb, u.String(), nil)
	return req
}

type securityError struct {
	Message string `json:"message"`
	Code    int    `json:"-"`
}

func (e securityError) Error() string {
	return e.Message
}

// NewSecurityUserAPI constructs a new SecurityUserAPI that uses HTTP to
// interact with etcd's user creation and modification features.
func NewSecurityUserAPI(c Client) SecurityUserAPI {
	return &httpSecurityUserAPI{
		client: c,
	}
}

type SecurityUserAPI interface {
	// Add a user.
	AddUser(ctx context.Context, username string, password string) error

	// Remove a user.
	RemoveUser(ctx context.Context, username string) error

	// Get user details.
	GetUser(ctx context.Context, username string) (*User, error)

	// Grant a user some permission roles.
	GrantUser(ctx context.Context, username string, roles []string) (*User, error)

	// Revoke some permission roles from a user.
	RevokeUser(ctx context.Context, username string, roles []string) (*User, error)

	// Change the user's password.
	ChangePassword(ctx context.Context, username string, password string) (*User, error)

	// List users.
	ListUsers(ctx context.Context) ([]string, error)
}

type httpSecurityUserAPI struct {
	client httpClient
}

type securityUserAPIAction struct {
	verb     string
	username string
	user     *User
}

type securityUserAPIList struct{}

func (list *securityUserAPIList) HTTPRequest(ep url.URL) *http.Request {
	u := v2SecurityURL(ep, "users", "")
	req, _ := http.NewRequest("GET", u.String(), nil)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (l *securityUserAPIAction) HTTPRequest(ep url.URL) *http.Request {
	u := v2SecurityURL(ep, "users", l.username)
	if l.user == nil {
		req, _ := http.NewRequest(l.verb, u.String(), nil)
		return req
	}
	b, err := json.Marshal(l.user)
	if err != nil {
		panic(err)
	}
	body := bytes.NewReader(b)
	req, _ := http.NewRequest(l.verb, u.String(), body)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func (u *httpSecurityUserAPI) ListUsers(ctx context.Context) ([]string, error) {
	resp, body, err := u.client.Do(ctx, &securityUserAPIList{})
	if err != nil {
		return nil, err
	}
	if err := assertStatusCode(resp.StatusCode, http.StatusOK); err != nil {
		var sec securityError
		err := json.Unmarshal(body, &sec)
		if err != nil {
			return nil, err
		}
		return nil, sec
	}
	var userList struct {
		Users []string `json:"users"`
	}
	err = json.Unmarshal(body, &userList)
	if err != nil {
		return nil, err
	}
	return userList.Users, nil
}

func (u *httpSecurityUserAPI) AddUser(ctx context.Context, username string, password string) error {
	user := &User{
		User:     username,
		Password: password,
	}
	return u.addRemoveUser(ctx, &securityUserAPIAction{
		verb:     "PUT",
		username: username,
		user:     user,
	})
}

func (u *httpSecurityUserAPI) RemoveUser(ctx context.Context, username string) error {
	return u.addRemoveUser(ctx, &securityUserAPIAction{
		verb:     "DELETE",
		username: username,
	})
}

func (u *httpSecurityUserAPI) addRemoveUser(ctx context.Context, req *securityUserAPIAction) error {
	resp, body, err := u.client.Do(ctx, req)
	if err != nil {
		return err
	}
	if err := assertStatusCode(resp.StatusCode, http.StatusOK, http.StatusCreated); err != nil {
		var sec securityError
		err := json.Unmarshal(body, &sec)
		if err != nil {
			return err
		}
		return sec
	}
	return nil
}

func (u *httpSecurityUserAPI) GetUser(ctx context.Context, username string) (*User, error) {
	return u.modUser(ctx, &securityUserAPIAction{
		verb:     "GET",
		username: username,
	})
}

func (u *httpSecurityUserAPI) GrantUser(ctx context.Context, username string, roles []string) (*User, error) {
	user := &User{
		User:  username,
		Grant: roles,
	}
	return u.modUser(ctx, &securityUserAPIAction{
		verb:     "PUT",
		username: username,
		user:     user,
	})
}

func (u *httpSecurityUserAPI) RevokeUser(ctx context.Context, username string, roles []string) (*User, error) {
	user := &User{
		User:   username,
		Revoke: roles,
	}
	return u.modUser(ctx, &securityUserAPIAction{
		verb:     "PUT",
		username: username,
		user:     user,
	})
}

func (u *httpSecurityUserAPI) ChangePassword(ctx context.Context, username string, password string) (*User, error) {
	user := &User{
		User:     username,
		Password: password,
	}
	return u.modUser(ctx, &securityUserAPIAction{
		verb:     "PUT",
		username: username,
		user:     user,
	})
}

func (u *httpSecurityUserAPI) modUser(ctx context.Context, req *securityUserAPIAction) (*User, error) {
	resp, body, err := u.client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := assertStatusCode(resp.StatusCode, http.StatusOK); err != nil {
		var sec securityError
		err := json.Unmarshal(body, &sec)
		if err != nil {
			return nil, err
		}
		return nil, sec
	}
	var user User
	err = json.Unmarshal(body, &user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
