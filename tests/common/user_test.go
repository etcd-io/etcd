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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestUserAdd_Simple(t *testing.T) {
	testRunner.BeforeTest(t)
	tcs := []struct {
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
	for _, tc := range clusterTestCases() {
		for _, nc := range tcs {
			t.Run(tc.name+"/"+nc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
				defer clus.Close()
				cc := testutils.MustClient(clus.Client())

				testutils.ExecuteUntil(ctx, t, func() {
					resp, err := cc.UserAdd(ctx, nc.username, nc.password, config.UserAddOptions{NoPassword: nc.noPassword})
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
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				user := "barb"
				password := "rhubarb"

				_, err := cc.UserAdd(ctx, user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("first user creation should succeed, err: %v", err)
				}

				_, err = cc.UserAdd(ctx, user, password, config.UserAddOptions{})
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
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				// No Users Yet
				resp, err := cc.UserList(ctx)
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 0 {
					t.Fatalf("expected no pre-existing users, found: %q", resp.Users)
				}

				user := "barb"
				password := "rhubarb"

				_, err = cc.UserAdd(ctx, user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("user creation should succeed, err: %v", err)
				}

				// Users!
				resp, err = cc.UserList(ctx)
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
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				user := "barb"
				password := "rhubarb"

				_, err := cc.UserAdd(ctx, user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("user creation should succeed, err: %v", err)
				}

				resp, err := cc.UserList(ctx)
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 1 {
					t.Fatalf("expected one user, found: %q", resp.Users)
				}

				// Delete barb, sorry barb!
				_, err = cc.UserDelete(ctx, user)
				if err != nil {
					t.Fatalf("user deletion should succeed at first, err: %v", err)
				}

				resp, err = cc.UserList(ctx)
				if err != nil {
					t.Fatalf("user listing should succeed, err: %v", err)
				}
				if len(resp.Users) != 0 {
					t.Fatalf("expected no users after deletion, found: %q", resp.Users)
				}

				// Try to delete barb again
				_, err = cc.UserDelete(ctx, user)
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
	for _, tc := range clusterTestCases() {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clus := testRunner.NewCluster(ctx, t, config.WithClusterConfig(tc.config))
			defer clus.Close()
			cc := testutils.MustClient(clus.Client())

			testutils.ExecuteUntil(ctx, t, func() {
				user := "barb"
				password := "rhubarb"
				newPassword := "potato"

				_, err := cc.UserAdd(ctx, user, password, config.UserAddOptions{})
				if err != nil {
					t.Fatalf("user creation should succeed, err: %v", err)
				}

				err = cc.UserChangePass(ctx, user, newPassword)
				if err != nil {
					t.Fatalf("user password change should succeed, err: %v", err)
				}

				err = cc.UserChangePass(ctx, "non-existent-user", newPassword)
				if err == nil {
					t.Fatalf("user password change for non-existent user should fail")
				}
				assert.Contains(t, err.Error(), "user name not found")
			})
		})
	}
}
