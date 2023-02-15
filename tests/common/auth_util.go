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
	"fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/interfaces"
)

const (
	RootUserName = "root"
	RootRoleName = "root"
	RootPassword = "rootPassword"
	TestUserName = "test-user"
	TestRoleName = "test-role"
	TestPassword = "pass"
)

var (
	RootUser = AuthUser{User: RootUserName, Pass: RootPassword, Role: RootRoleName}
	TestUser = AuthUser{User: TestUserName, Pass: TestPassword, Role: TestRoleName}

	TestRole = AuthRole{
		Role:       TestRoleName,
		Permission: clientv3.PermissionType(clientv3.PermReadWrite),
		Key:        "foo",
		KeyEnd:     "",
	}
)

type AuthRole struct {
	Role       string
	Permission clientv3.PermissionType
	Key        string
	KeyEnd     string
}

type AuthUser struct {
	User string
	Pass string
	Role string
}

func createRoles(c interfaces.Client, roles []AuthRole) error {
	for _, r := range roles {
		// add role
		if _, err := c.RoleAdd(context.TODO(), r.Role); err != nil {
			return fmt.Errorf("RoleAdd failed: %w", err)
		}

		// grant permission to role
		if _, err := c.RoleGrantPermission(context.TODO(), r.Role, r.Key, r.KeyEnd, r.Permission); err != nil {
			return fmt.Errorf("RoleGrantPermission failed: %w", err)
		}
	}

	return nil
}

func createUsers(c interfaces.Client, users []AuthUser) error {
	for _, u := range users {
		// add user
		if _, err := c.UserAdd(context.TODO(), u.User, u.Pass, config.UserAddOptions{}); err != nil {
			return fmt.Errorf("UserAdd failed: %w", err)
		}

		// grant role to user
		if _, err := c.UserGrantRole(context.TODO(), u.User, u.Role); err != nil {
			return fmt.Errorf("UserGrantRole failed: %w", err)
		}
	}

	return nil
}

func setupAuth(c interfaces.Client, roles []AuthRole, users []AuthUser) error {
	// create roles
	if err := createRoles(c, roles); err != nil {
		return err
	}

	if err := createUsers(c, users); err != nil {
		return err
	}

	// enable auth
	if err := c.AuthEnable(context.TODO()); err != nil {
		return err
	}

	return nil
}
