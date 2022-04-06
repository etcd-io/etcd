// Copyright 2022 The etcd Authors
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

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestUserAdd_Simple(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		nestedCases := []struct {
			name          string
			username      string
			password      string
			noPassword    bool
			expectedError string
		}{
			{
				name:     "empty_username_not_allowed",
				username: "",
				password: "foobar",
				// Very Vague error expectation because the CLI and the API return very
				// different error structures.
				expectedError: "user name",
			},
			{
				// Can create a user with no password, restricted to CN auth
				name:       "no_password_with_noPassword_set",
				username:   "foo",
				password:   "",
				noPassword: true,
			},
			{
				// Can create a user with no password, but not restricted to CN auth
				name:       "no_password_without_noPassword_set",
				username:   "foo",
				password:   "",
				noPassword: false,
			},
			{
				name:     "regular_user_with_password",
				username: "foo",
				password: "bar",
			},
		}
		for _, nc := range nestedCases {
			t.Run(tc.name+"/"+nc.name, func(t *testing.T) {
				clus := testRunner.NewCluster(t, tc.config)
				defer clus.Close()
				cc := clus.Client()

				testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
					resp, err := cc.UserAdd(nc.username, nc.password, config.UserAddOptions{NoPassword: nc.noPassword})
					if nc.expectedError != "" {
						if err != nil {
							assert.Contains(t, err.Error(), nc.expectedError)
							return
						}

						t.Fatalf("expected user creation to fail")
					}

					if err != nil {
						t.Fatalf("expected no error, err: %v", err)
					}

					if resp == nil {
						t.Fatalf("unexpected nil response to successful user creation")
					}
				})
			})
		}
	}
}

func TestUserAdd_DuplicateUserNotAllowed(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				user := "barb"
				password := "rhubarb"

				_, err := cc.UserAdd(user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("first user creation should succeed, err: %v", err)
				}

				_, err = cc.UserAdd(user, password, config.UserAddOptions{})
				if err == nil {
					t.Fatalf("duplicate user creation should fail")
				}
				assert.Contains(t, err.Error(), "etcdserver: user name already exists")
			})
		})
	}
}

func TestUserList(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				// No Users Yet
				resp, err := cc.UserList()
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 0 {
					t.Fatalf("expected no pre-existing users, found: %q", resp.Users)
				}

				user := "barb"
				password := "rhubarb"

				_, err = cc.UserAdd(user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("user creation should succeed, err: %v", err)
				}

				// Users!
				resp, err = cc.UserList()
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 1 {
					t.Fatalf("expected one user, found: %q", resp.Users)
				}
			})
		})
	}
}

func TestUserDelete(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				user := "barb"
				password := "rhubarb"

				_, err := cc.UserAdd(user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("user creation should succeed, err: %v", err)
				}

				resp, err := cc.UserList()
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 1 {
					t.Fatalf("expected one user, found: %q", resp.Users)
				}

				// Delete barb, sorry barb!
				_, err = cc.UserDelete(user)
				if err != nil {
					t.Fatalf("user deletion should succeed at first, err: %v", err)
				}

				resp, err = cc.UserList()
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 0 {
					t.Fatalf("expected no users after deletion, found: %q", resp.Users)
				}

				// Try to delete barb again
				_, err = cc.UserDelete(user)
				if err == nil {
					t.Fatalf("deleting a non-existent user should fail")
				}
				assert.Contains(t, err.Error(), "user name not found")
			})
		})
	}
}

func TestUserChangePassword(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				user := "barb"
				password := "rhubarb"
				newPassword := "potato"

				_, err := cc.UserAdd(user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("user creation should succeed, err: %v", err)
				}

				err = cc.UserChangePass(user, newPassword)
				if err != nil {
					t.Fatalf("user password change should succeed, err: %v", err)
				}

				err = cc.UserChangePass("non-existent-user", newPassword)
				if err == nil {
					t.Fatalf("user password change for non-existent user should fail")
				}
				assert.Contains(t, err.Error(), "user name not found")
			})
		})
	}
}
