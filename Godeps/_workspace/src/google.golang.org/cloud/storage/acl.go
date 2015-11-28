// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"fmt"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	raw "google.golang.org/api/storage/v1"
)

// ACLRole is the the access permission for the entity.
type ACLRole string

const (
	RoleOwner  ACLRole = "OWNER"
	RoleReader ACLRole = "READER"
)

// ACLEntity is an entity holding an ACL permission.
//
// It could be in the form of:
// "user-<userId>", "user-<email>","group-<groupId>", "group-<email>",
// "domain-<domain>" and "project-team-<projectId>".
//
// Or one of the predefined constants: AllUsers, AllAuthenticatedUsers.
type ACLEntity string

const (
	AllUsers              ACLEntity = "allUsers"
	AllAuthenticatedUsers ACLEntity = "allAuthenticatedUsers"
)

// ACLRule represents an access control list rule entry for a Google Cloud Storage object or bucket.
// A bucket is a Google Cloud Storage container whose name is globally unique and contains zero or
// more objects.  An object is a blob of data that is stored in a bucket.
type ACLRule struct {
	// Entity identifies the entity holding the current rule's permissions.
	Entity ACLEntity

	// Role is the the access permission for the entity.
	Role ACLRole
}

// DefaultACL returns the default object ACL entries for the named bucket.
func DefaultACL(ctx context.Context, bucket string) ([]ACLRule, error) {
	acls, err := rawService(ctx).DefaultObjectAccessControls.List(bucket).Do()
	if err != nil {
		return nil, fmt.Errorf("storage: error listing default object ACL for bucket %q: %v", bucket, err)
	}
	r := make([]ACLRule, 0, len(acls.Items))
	for _, v := range acls.Items {
		if m, ok := v.(map[string]interface{}); ok {
			entity, ok1 := m["entity"].(string)
			role, ok2 := m["role"].(string)
			if ok1 && ok2 {
				r = append(r, ACLRule{Entity: ACLEntity(entity), Role: ACLRole(role)})
			}
		}
	}
	return r, nil
}

// PutDefaultACLRule saves the named default object ACL entity with the provided role for the named bucket.
func PutDefaultACLRule(ctx context.Context, bucket string, entity ACLEntity, role ACLRole) error {
	acl := &raw.ObjectAccessControl{
		Bucket: bucket,
		Entity: string(entity),
		Role:   string(role),
	}
	_, err := rawService(ctx).DefaultObjectAccessControls.Update(bucket, string(entity), acl).Do()
	if err != nil {
		return fmt.Errorf("storage: error updating default ACL rule for bucket %q, entity %q: %v", bucket, entity, err)
	}
	return nil
}

// DeleteDefaultACLRule deletes the named default ACL entity for the named bucket.
func DeleteDefaultACLRule(ctx context.Context, bucket string, entity ACLEntity) error {
	err := rawService(ctx).DefaultObjectAccessControls.Delete(bucket, string(entity)).Do()
	if err != nil {
		return fmt.Errorf("storage: error deleting default ACL rule for bucket %q, entity %q: %v", bucket, entity, err)
	}
	return nil
}

// BucketACL returns the ACL entries for the named bucket.
func BucketACL(ctx context.Context, bucket string) ([]ACLRule, error) {
	acls, err := rawService(ctx).BucketAccessControls.List(bucket).Do()
	if err != nil {
		return nil, fmt.Errorf("storage: error listing bucket ACL for bucket %q: %v", bucket, err)
	}
	r := make([]ACLRule, len(acls.Items))
	for i, v := range acls.Items {
		r[i].Entity = ACLEntity(v.Entity)
		r[i].Role = ACLRole(v.Role)
	}
	return r, nil
}

// PutBucketACLRule saves the named ACL entity with the provided role for the named bucket.
func PutBucketACLRule(ctx context.Context, bucket string, entity ACLEntity, role ACLRole) error {
	acl := &raw.BucketAccessControl{
		Bucket: bucket,
		Entity: string(entity),
		Role:   string(role),
	}
	_, err := rawService(ctx).BucketAccessControls.Update(bucket, string(entity), acl).Do()
	if err != nil {
		return fmt.Errorf("storage: error updating bucket ACL rule for bucket %q, entity %q: %v", bucket, entity, err)
	}
	return nil
}

// DeleteBucketACLRule deletes the named ACL entity for the named bucket.
func DeleteBucketACLRule(ctx context.Context, bucket string, entity ACLEntity) error {
	err := rawService(ctx).BucketAccessControls.Delete(bucket, string(entity)).Do()
	if err != nil {
		return fmt.Errorf("storage: error deleting bucket ACL rule for bucket %q, entity %q: %v", bucket, entity, err)
	}
	return nil
}

// ACL returns the ACL entries for the named object.
func ACL(ctx context.Context, bucket, object string) ([]ACLRule, error) {
	acls, err := rawService(ctx).ObjectAccessControls.List(bucket, object).Do()
	if err != nil {
		return nil, fmt.Errorf("storage: error listing object ACL for bucket %q, file %q: %v", bucket, object, err)
	}
	r := make([]ACLRule, 0, len(acls.Items))
	for _, v := range acls.Items {
		if m, ok := v.(map[string]interface{}); ok {
			entity, ok1 := m["entity"].(string)
			role, ok2 := m["role"].(string)
			if ok1 && ok2 {
				r = append(r, ACLRule{Entity: ACLEntity(entity), Role: ACLRole(role)})
			}
		}
	}
	return r, nil
}

// PutACLRule saves the named ACL entity with the provided role for the named object.
func PutACLRule(ctx context.Context, bucket, object string, entity ACLEntity, role ACLRole) error {
	acl := &raw.ObjectAccessControl{
		Bucket: bucket,
		Entity: string(entity),
		Role:   string(role),
	}
	_, err := rawService(ctx).ObjectAccessControls.Update(bucket, object, string(entity), acl).Do()
	if err != nil {
		return fmt.Errorf("storage: error updating object ACL rule for bucket %q, file %q, entity %q: %v", bucket, object, entity, err)
	}
	return nil
}

// DeleteACLRule deletes the named ACL entity for the named object.
func DeleteACLRule(ctx context.Context, bucket, object string, entity ACLEntity) error {
	err := rawService(ctx).ObjectAccessControls.Delete(bucket, object, string(entity)).Do()
	if err != nil {
		return fmt.Errorf("storage: error deleting object ACL rule for bucket %q, file %q, entity %q: %v", bucket, object, entity, err)
	}
	return nil
}
