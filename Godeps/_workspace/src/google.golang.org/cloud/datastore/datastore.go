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

// Package datastore contains a Google Cloud Datastore client.
package datastore

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/golang/protobuf/proto"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/internal"
	pb "github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/internal/datastore"
)

// ContextKey represents a context key specific to the datastore
type ContextKey string

const (
	// ScopeDatastore grants permissions to view and/or manage datastore entities
	ScopeDatastore = "https://www.googleapis.com/auth/datastore"

	// ScopeUserEmail grants permission to view the user's email address.
	// It is required to access the datastore
	ScopeUserEmail = "https://www.googleapis.com/auth/userinfo.email"
)

var (
	// ErrInvalidEntityType is returned when functions like Get or Next are
	// passed a dst or src argument of invalid type.
	ErrInvalidEntityType = errors.New("datastore: invalid entity type")
	// ErrInvalidKey is returned when an invalid key is presented.
	ErrInvalidKey = errors.New("datastore: invalid key")
	// ErrNoSuchEntity is returned when no entity was found for a given key.
	ErrNoSuchEntity = errors.New("datastore: no such entity")
)

type multiArgType int

const (
	multiArgTypeInvalid multiArgType = iota
	multiArgTypePropertyLoadSaver
	multiArgTypeStruct
	multiArgTypeStructPtr
	multiArgTypeInterface
)

// nsKey is the type of the context.Context key to store the datastore
// namespace.
type nsKey struct{}

// WithNamespace returns a new context that limits the scope its parent
// context with a Datastore namespace.
func WithNamespace(parent context.Context, namespace string) context.Context {
	return context.WithValue(parent, nsKey{}, namespace)
}

// ctxNamespace returns the active namespace for a context.
// It defaults to "" if no namespace was specified.
func ctxNamespace(ctx context.Context) string {
	v, _ := ctx.Value(nsKey{}).(string)
	return v
}

// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct.
// StructType is the type of the struct pointed to by the destination argument
// passed to Get or to Iterator.Next.
type ErrFieldMismatch struct {
	StructType reflect.Type
	FieldName  string
	Reason     string
}

// errHTTP is returned when responds is a non-200 HTTP response.
type errHTTP struct {
	StatusCode int
	Body       string
	err        error
}

func (e *errHTTP) Error() string {
	if e.err == nil {
		return fmt.Sprintf("error during call, http status code: %v %s", e.StatusCode, e.Body)
	}
	return e.err.Error()
}

func (e *ErrFieldMismatch) Error() string {
	return fmt.Sprintf("datastore: cannot load field %q into a %q: %s",
		e.FieldName, e.StructType, e.Reason)
}

// baseUrl gets the base url active for the datastore service
// defaults to "https://www.googleapis.com/datastore/v1beta2/datasets/" if none was specified
func baseUrl(ctx context.Context) string {
	v := ctx.Value(ContextKey("base_url"))
	if v == nil {
		return "https://www.googleapis.com/datastore/v1beta2/datasets/"
	} else {
		return v.(string)
	}
}

func call(ctx context.Context, method string, req proto.Message, resp proto.Message) error {
	payload, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	url := baseUrl(ctx) + internal.ProjID(ctx) + "/" + method
	r, err := internal.HTTPClient(ctx).Post(url, "application/x-protobuf", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	all, err := ioutil.ReadAll(r.Body)
	if r.StatusCode != http.StatusOK {
		e := &errHTTP{
			StatusCode: r.StatusCode,
			err:        err,
		}
		if err == nil {
			e.Body = string(all)
		}
		return e
	}
	if err != nil {
		return err
	}
	if err = proto.Unmarshal(all, resp); err != nil {
		return err
	}
	return nil
}

func keyToProto(k *Key) *pb.Key {
	if k == nil {
		return nil
	}

	// TODO(jbd): Eliminate unrequired allocations.
	path := []*pb.Key_PathElement(nil)
	for {
		el := &pb.Key_PathElement{
			Kind: proto.String(k.kind),
		}
		if k.id != 0 {
			el.Id = proto.Int64(k.id)
		}
		if k.name != "" {
			el.Name = proto.String(k.name)
		}
		path = append([]*pb.Key_PathElement{el}, path...)
		if k.parent == nil {
			break
		}
		k = k.parent
	}
	key := &pb.Key{
		PathElement: path,
	}
	if k.namespace != "" {
		key.PartitionId = &pb.PartitionId{
			Namespace: proto.String(k.namespace),
		}
	}
	return key
}

func protoToKey(p *pb.Key) *Key {
	keys := make([]*Key, len(p.GetPathElement()))
	for i, el := range p.GetPathElement() {
		keys[i] = &Key{
			namespace: p.GetPartitionId().GetNamespace(),
			kind:      el.GetKind(),
			id:        el.GetId(),
			name:      el.GetName(),
		}
	}
	for i := 0; i < len(keys)-1; i++ {
		keys[i+1].parent = keys[i]
	}
	return keys[len(keys)-1]
}

// multiKeyToProto is a batch version of keyToProto.
func multiKeyToProto(keys []*Key) []*pb.Key {
	ret := make([]*pb.Key, len(keys))
	for i, k := range keys {
		ret[i] = keyToProto(k)
	}
	return ret
}

// multiKeyToProto is a batch version of keyToProto.
func multiProtoToKey(keys []*pb.Key) []*Key {
	ret := make([]*Key, len(keys))
	for i, k := range keys {
		ret[i] = protoToKey(k)
	}
	return ret
}

// multiValid is a batch version of Key.valid. It returns an error, not a
// []bool.
func multiValid(key []*Key) error {
	invalid := false
	for _, k := range key {
		if !k.valid() {
			invalid = true
			break
		}
	}
	if !invalid {
		return nil
	}
	err := make(MultiError, len(key))
	for i, k := range key {
		if !k.valid() {
			err[i] = ErrInvalidKey
		}
	}
	return err
}

// checkMultiArg checks that v has type []S, []*S, []I, or []P, for some struct
// type S, for some interface type I, or some non-interface non-pointer type P
// such that P or *P implements PropertyLoadSaver.
//
// It returns what category the slice's elements are, and the reflect.Type
// that represents S, I or P.
//
// As a special case, PropertyList is an invalid type for v.
func checkMultiArg(v reflect.Value) (m multiArgType, elemType reflect.Type) {
	if v.Kind() != reflect.Slice {
		return multiArgTypeInvalid, nil
	}
	if v.Type() == typeOfPropertyList {
		return multiArgTypeInvalid, nil
	}
	elemType = v.Type().Elem()
	if reflect.PtrTo(elemType).Implements(typeOfPropertyLoadSaver) {
		return multiArgTypePropertyLoadSaver, elemType
	}
	switch elemType.Kind() {
	case reflect.Struct:
		return multiArgTypeStruct, elemType
	case reflect.Interface:
		return multiArgTypeInterface, elemType
	case reflect.Ptr:
		elemType = elemType.Elem()
		if elemType.Kind() == reflect.Struct {
			return multiArgTypeStructPtr, elemType
		}
	}
	return multiArgTypeInvalid, nil
}

// Get loads the entity stored for k into dst, which must be a struct pointer
// or implement PropertyLoadSaver. If there is no such entity for the key, Get
// returns ErrNoSuchEntity.
//
// The values of dst's unmatched struct fields are not modified, and matching
// slice-typed fields are not reset before appending to them. In particular, it
// is recommended to pass a pointer to a zero valued struct on each Get call.
//
// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct. ErrFieldMismatch is only returned if
// dst is a struct pointer.
func Get(ctx context.Context, key *Key, dst interface{}) error {
	err := get(ctx, []*Key{key}, []interface{}{dst}, nil)
	if me, ok := err.(MultiError); ok {
		return me[0]
	}
	return err
}

// GetMulti is a batch version of Get.
//
// dst must be a []S, []*S, []I or []P, for some struct type S, some interface
// type I, or some non-interface non-pointer type P such that P or *P
// implements PropertyLoadSaver. If an []I, each element must be a valid dst
// for Get: it must be a struct pointer or implement PropertyLoadSaver.
//
// As a special case, PropertyList is an invalid type for dst, even though a
// PropertyList is a slice of structs. It is treated as invalid to avoid being
// mistakenly passed when []PropertyList was intended.
func GetMulti(ctx context.Context, keys []*Key, dst interface{}) error {
	return get(ctx, keys, dst, nil)
}

func get(ctx context.Context, keys []*Key, dst interface{}, opts *pb.ReadOptions) error {
	v := reflect.ValueOf(dst)
	multiArgType, _ := checkMultiArg(v)

	// Sanity checks
	if multiArgType == multiArgTypeInvalid {
		return errors.New("datastore: dst has invalid type")
	}
	if len(keys) != v.Len() {
		return errors.New("datastore: keys and dst slices have different length")
	}
	if len(keys) == 0 {
		return nil
	}

	// Go through keys, validate them, serialize then, and create a dict mapping them to their index
	multiErr, any := make(MultiError, len(keys)), false
	keyMap := make(map[string]int)
	pbKeys := make([]*pb.Key, len(keys))
	for i, k := range keys {
		if !k.valid() {
			multiErr[i] = ErrInvalidKey
			any = true
		} else {
			keyMap[k.String()] = i
			pbKeys[i] = keyToProto(k)
		}
	}
	if any {
		return multiErr
	}
	req := &pb.LookupRequest{
		Key:         pbKeys,
		ReadOptions: opts,
	}
	resp := &pb.LookupResponse{}
	if err := call(ctx, "lookup", req, resp); err != nil {
		return err
	}
	if len(resp.Deferred) > 0 {
		// TODO(jbd): Assess whether we should retry the deferred keys.
		return errors.New("datastore: some entities temporarily unavailable")
	}
	if len(keys) != len(resp.Found)+len(resp.Missing) {
		return errors.New("datastore: internal error: server returned the wrong number of entities")
	}
	for _, e := range resp.Found {
		k := protoToKey(e.Entity.Key)
		index := keyMap[k.String()]
		elem := v.Index(index)
		if multiArgType == multiArgTypePropertyLoadSaver || multiArgType == multiArgTypeStruct {
			elem = elem.Addr()
		}
		err := loadEntity(elem.Interface(), e.Entity)
		if err != nil {
			multiErr[index] = err
			any = true
		}
	}
	for _, e := range resp.Missing {
		k := protoToKey(e.Entity.Key)
		multiErr[keyMap[k.String()]] = ErrNoSuchEntity
		any = true
	}
	if any {
		return multiErr
	}
	return nil
}

// Put saves the entity src into the datastore with key k. src must be a struct
// pointer or implement PropertyLoadSaver; if a struct pointer then any
// unexported fields of that struct will be skipped. If k is an incomplete key,
// the returned key will be a unique key generated by the datastore.
func Put(ctx context.Context, key *Key, src interface{}) (*Key, error) {
	k, err := PutMulti(ctx, []*Key{key}, []interface{}{src})
	if err != nil {
		if me, ok := err.(MultiError); ok {
			return nil, me[0]
		}
		return nil, err
	}
	return k[0], nil
}

// PutMulti is a batch version of Put.
//
// src must satisfy the same conditions as the dst argument to GetMulti.
func PutMulti(ctx context.Context, keys []*Key, src interface{}) ([]*Key, error) {
	mutation, err := putMutation(keys, src)
	if err != nil {
		return nil, err
	}

	// Make the request.
	req := &pb.CommitRequest{
		Mutation: mutation,
		Mode:     pb.CommitRequest_NON_TRANSACTIONAL.Enum(),
	}
	resp := &pb.CommitResponse{}
	if err := call(ctx, "commit", req, resp); err != nil {
		return nil, err
	}

	// Copy any newly minted keys into the returned keys.
	newKeys := make(map[int]int) // Map of index in returned slice to index in response.
	ret := make([]*Key, len(keys))
	var idx int
	for i, key := range keys {
		if key.Incomplete() {
			// This key will be in the mutation result.
			newKeys[i] = idx
			idx++
		} else {
			ret[i] = key
		}
	}
	if len(newKeys) != len(resp.MutationResult.InsertAutoIdKey) {
		return nil, errors.New("datastore: internal error: server returned the wrong number of keys")
	}
	for retI, respI := range newKeys {
		ret[retI] = protoToKey(resp.MutationResult.InsertAutoIdKey[respI])
	}
	return ret, nil
}

func putMutation(keys []*Key, src interface{}) (*pb.Mutation, error) {
	v := reflect.ValueOf(src)
	multiArgType, _ := checkMultiArg(v)
	if multiArgType == multiArgTypeInvalid {
		return nil, errors.New("datastore: src has invalid type")
	}
	if len(keys) != v.Len() {
		return nil, errors.New("datastore: key and src slices have different length")
	}
	if len(keys) == 0 {
		return nil, nil
	}
	if err := multiValid(keys); err != nil {
		return nil, err
	}
	var upsert, insert []*pb.Entity
	for i, k := range keys {
		val := reflect.ValueOf(src).Index(i)
		// If src is an interface slice []interface{}{ent1, ent2}
		if val.Kind() == reflect.Interface && val.Elem().Kind() == reflect.Slice {
			val = val.Elem()
		}
		// If src is a slice of ptrs []*T{ent1, ent2}
		if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Slice {
			val = val.Elem()
		}
		p, err := saveEntity(k, val.Interface())
		if err != nil {
			return nil, fmt.Errorf("datastore: Error while saving %v: %v", k.String(), err)
		}
		if k.Incomplete() {
			insert = append(insert, p)
		} else {
			upsert = append(upsert, p)
		}
	}

	return &pb.Mutation{
		InsertAutoId: insert,
		Upsert:       upsert,
	}, nil
}

// Delete deletes the entity for the given key.
func Delete(ctx context.Context, key *Key) error {
	err := DeleteMulti(ctx, []*Key{key})
	if me, ok := err.(MultiError); ok {
		return me[0]
	}
	return err
}

// DeleteMulti is a batch version of Delete.
func DeleteMulti(ctx context.Context, keys []*Key) error {
	mutation, err := deleteMutation(keys)
	if err != nil {
		return err
	}

	req := &pb.CommitRequest{
		Mutation: mutation,
		Mode:     pb.CommitRequest_NON_TRANSACTIONAL.Enum(),
	}
	resp := &pb.CommitResponse{}
	return call(ctx, "commit", req, resp)
}

func deleteMutation(keys []*Key) (*pb.Mutation, error) {
	protoKeys := make([]*pb.Key, len(keys))
	for i, k := range keys {
		if k.Incomplete() {
			return nil, fmt.Errorf("datastore: can't delete the incomplete key: %v", k)
		}
		protoKeys[i] = keyToProto(k)
	}

	return &pb.Mutation{
		Delete: protoKeys,
	}, nil
}
