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

// +build integration

package datastore

import (
	"fmt"

	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/internal/testutil"
)

func TestBasics(t *testing.T) {
	type X struct {
		I int
		S string
		T time.Time
	}
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	x0 := X{66, "99", time.Now().Truncate(time.Millisecond)}
	k, err := Put(c, NewIncompleteKey(c, "BasicsX", nil), &x0)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	x1 := X{}
	err = Get(c, k, &x1)
	if err != nil {
		t.Errorf("Get: %v", err)
	}
	err = Delete(c, k)
	if err != nil {
		t.Errorf("Delete: %v", err)
	}
	if !reflect.DeepEqual(x0, x1) {
		t.Errorf("compare: x0=%v, x1=%v", x0, x1)
	}
}

func TestListValues(t *testing.T) {
	p0 := PropertyList{
		{Name: "L", Value: int64(12), Multiple: true},
		{Name: "L", Value: "string", Multiple: true},
		{Name: "L", Value: true, Multiple: true},
	}
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	k, err := Put(c, NewIncompleteKey(c, "ListValue", nil), &p0)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	var p1 PropertyList
	if err := Get(c, k, &p1); err != nil {
		t.Errorf("Get: %v", err)
	}
	if !reflect.DeepEqual(p0, p1) {
		t.Errorf("compare:\np0=%v\np1=%#v", p0, p1)
	}
	if err = Delete(c, k); err != nil {
		t.Errorf("Delete: %v", err)
	}
}

func TestGetMulti(t *testing.T) {
	type X struct {
		I int
	}
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	p := NewKey(c, "X", "", time.Now().Unix(), nil)

	cases := []struct {
		key *Key
		put bool
	}{
		{key: NewKey(c, "X", "item1", 0, p), put: true},
		{key: NewKey(c, "X", "item2", 0, p), put: false},
		{key: NewKey(c, "X", "item3", 0, p), put: false},
		{key: NewKey(c, "X", "item4", 0, p), put: true},
	}

	var src, dst []*X
	var srcKeys, dstKeys []*Key
	for _, c := range cases {
		dst = append(dst, &X{})
		dstKeys = append(dstKeys, c.key)
		if c.put {
			src = append(src, &X{})
			srcKeys = append(srcKeys, c.key)
		}
	}
	if _, err := PutMulti(c, srcKeys, src); err != nil {
		t.Error(err)
	}
	err := GetMulti(c, dstKeys, dst)
	if err == nil {
		t.Errorf("GetMulti got %v, expected error", err)
	}
	e, ok := err.(MultiError)
	if !ok {
		t.Errorf("GetMulti got %t, expected MultiError", err)
	}
	for i, err := range e {
		got, want := err, (error)(nil)
		if !cases[i].put {
			got, want = err, ErrNoSuchEntity
		}
		if got != want {
			t.Errorf("MultiError[%d] == %v, want %v", i, got, want)
		}
	}
}

type Z struct {
	S string
	T string `datastore:",noindex"`
	P []byte
	K []byte `datastore:",noindex"`
}

func (z Z) String() string {
	var lens []string
	v := reflect.ValueOf(z)
	for i := 0; i < v.NumField(); i++ {
		if l := v.Field(i).Len(); l > 0 {
			lens = append(lens, fmt.Sprintf("len(%s)=%d", v.Type().Field(i).Name, l))
		}
	}
	return fmt.Sprintf("Z{ %s }", strings.Join(lens, ","))
}

func TestUnindexableValues(t *testing.T) {
	x500 := strings.Repeat("x", 500)
	x501 := strings.Repeat("x", 501)
	testCases := []struct {
		in      Z
		wantErr bool
	}{
		{in: Z{S: x500}, wantErr: false},
		{in: Z{S: x501}, wantErr: true},
		{in: Z{T: x500}, wantErr: false},
		{in: Z{T: x501}, wantErr: false},
		{in: Z{P: []byte(x500)}, wantErr: false},
		{in: Z{P: []byte(x501)}, wantErr: true},
		{in: Z{K: []byte(x500)}, wantErr: false},
		{in: Z{K: []byte(x501)}, wantErr: false},
	}
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	for _, tt := range testCases {
		_, err := Put(c, NewIncompleteKey(c, "BasicsZ", nil), &tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("Put %s got err %v, want err %t", tt.in, err, tt.wantErr)
		}
	}
}

type SQChild struct {
	I, J int
	T, U int64
}

type SQTestCase struct {
	desc      string
	q         *Query
	wantCount int
	wantSum   int
}

func testSmallQueries(t *testing.T, c context.Context, parent *Key, children []*SQChild,
	testCases []SQTestCase, extraTests ...func()) {
	keys := make([]*Key, len(children))
	for i := range keys {
		keys[i] = NewIncompleteKey(c, "SQChild", parent)
	}
	keys, err := PutMulti(c, keys, children)
	if err != nil {
		t.Fatalf("PutMulti: %v", err)
	}
	defer func() {
		err := DeleteMulti(c, keys)
		if err != nil {
			t.Errorf("DeleteMulti: %v", err)
		}
	}()

	for _, tc := range testCases {
		count, err := tc.q.Count(c)
		if err != nil {
			t.Errorf("Count %q: %v", tc.desc, err)
			continue
		}
		if count != tc.wantCount {
			t.Errorf("Count %q: got %d want %d", tc.desc, count, tc.wantCount)
			continue
		}
	}

	for _, tc := range testCases {
		var got []SQChild
		_, err := tc.q.GetAll(c, &got)
		if err != nil {
			t.Errorf("GetAll %q: %v", tc.desc, err)
			continue
		}
		sum := 0
		for _, c := range got {
			sum += c.I + c.J
		}
		if sum != tc.wantSum {
			t.Errorf("sum %q: got %d want %d", tc.desc, sum, tc.wantSum)
			continue
		}
	}
	for _, x := range extraTests {
		x()
	}
}

func TestFilters(t *testing.T) {
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	parent := NewKey(c, "SQParent", "TestFilters", 0, nil)
	now := time.Now().Truncate(time.Millisecond).Unix()
	children := []*SQChild{
		{I: 0, T: now, U: now},
		{I: 1, T: now, U: now},
		{I: 2, T: now, U: now},
		{I: 3, T: now, U: now},
		{I: 4, T: now, U: now},
		{I: 5, T: now, U: now},
		{I: 6, T: now, U: now},
		{I: 7, T: now, U: now},
	}
	baseQuery := NewQuery("SQChild").Ancestor(parent).Filter("T=", now)
	testSmallQueries(t, c, parent, children, []SQTestCase{
		{
			"I>1",
			baseQuery.Filter("I>", 1),
			6,
			2 + 3 + 4 + 5 + 6 + 7,
		},
		{
			"I>2 AND I<=5",
			baseQuery.Filter("I>", 2).Filter("I<=", 5),
			3,
			3 + 4 + 5,
		},
		{
			"I>=3 AND I<3",
			baseQuery.Filter("I>=", 3).Filter("I<", 3),
			0,
			0,
		},
		{
			"I=4",
			baseQuery.Filter("I=", 4),
			1,
			4,
		},
	}, func() {
		got := []*SQChild{}
		want := []*SQChild{
			{I: 0, T: now, U: now},
			{I: 1, T: now, U: now},
			{I: 2, T: now, U: now},
			{I: 3, T: now, U: now},
			{I: 4, T: now, U: now},
			{I: 5, T: now, U: now},
			{I: 6, T: now, U: now},
			{I: 7, T: now, U: now},
		}
		_, err := baseQuery.Order("I").GetAll(c, &got)
		if err != nil {
			t.Errorf("GetAll: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("compare: got=%v, want=%v", got, want)
		}
	}, func() {
		got := []*SQChild{}
		want := []*SQChild{
			{I: 7, T: now, U: now},
			{I: 6, T: now, U: now},
			{I: 5, T: now, U: now},
			{I: 4, T: now, U: now},
			{I: 3, T: now, U: now},
			{I: 2, T: now, U: now},
			{I: 1, T: now, U: now},
			{I: 0, T: now, U: now},
		}
		_, err := baseQuery.Order("-I").GetAll(c, &got)
		if err != nil {
			t.Errorf("GetAll: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("compare: got=%v, want=%v", got, want)
		}
	})
}

func TestEventualConsistency(t *testing.T) {
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	parent := NewKey(c, "SQParent", "TestEventualConsistency", 0, nil)
	now := time.Now().Truncate(time.Millisecond).Unix()
	children := []*SQChild{
		{I: 0, T: now, U: now},
		{I: 1, T: now, U: now},
		{I: 2, T: now, U: now},
	}
	query := NewQuery("SQChild").Ancestor(parent).Filter("T =", now).EventualConsistency()
	testSmallQueries(t, c, parent, children, nil, func() {
		got, err := query.Count(c)
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if got < 0 || 3 < got {
			t.Errorf("Count: got %d, want [0,3]", got)
		}
	})
}

func TestProjection(t *testing.T) {
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	parent := NewKey(c, "SQParent", "TestProjection", 0, nil)
	now := time.Now().Truncate(time.Millisecond).Unix()
	children := []*SQChild{
		{I: 1 << 0, J: 100, T: now, U: now},
		{I: 1 << 1, J: 100, T: now, U: now},
		{I: 1 << 2, J: 200, T: now, U: now},
		{I: 1 << 3, J: 300, T: now, U: now},
		{I: 1 << 4, J: 300, T: now, U: now},
	}
	baseQuery := NewQuery("SQChild").Ancestor(parent).Filter("T=", now).Filter("J>", 150)
	testSmallQueries(t, c, parent, children, []SQTestCase{
		{
			"project",
			baseQuery.Project("J"),
			3,
			200 + 300 + 300,
		},
		{
			"distinct",
			baseQuery.Project("J").Distinct(),
			2,
			200 + 300,
		},
		{
			"project on meaningful (GD_WHEN) field",
			baseQuery.Project("U"),
			3,
			0,
		},
	})
}

func TestAllocateIDs(t *testing.T) {
	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	keys := make([]*Key, 5)
	for i := range keys {
		keys[i] = NewIncompleteKey(c, "AllocID", nil)
	}
	keys, err := AllocateIDs(c, keys)
	if err != nil {
		t.Errorf("AllocID #0 failed: %v", err)
	}
	if want := len(keys); want != 5 {
		t.Errorf("Expected to allocate 5 keys, %d keys are found", want)
	}
	for _, k := range keys {
		if k.Incomplete() {
			t.Errorf("Unexpeceted incomplete key found: %v", k)
		}
	}
}

func TestGetAllWithFieldMismatch(t *testing.T) {
	type Fat struct {
		X, Y int
	}
	type Thin struct {
		X int
	}

	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	putKeys := make([]*Key, 3)
	for i := range putKeys {
		putKeys[i] = NewKey(c, "GetAllThing", "", int64(10+i), nil)
		_, err := Put(c, putKeys[i], &Fat{X: 20 + i, Y: 30 + i})
		if err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	var got []Thin
	want := []Thin{
		{X: 20},
		{X: 21},
		{X: 22},
	}
	getKeys, err := NewQuery("GetAllThing").GetAll(c, &got)
	if len(getKeys) != 3 && !reflect.DeepEqual(getKeys, putKeys) {
		t.Errorf("GetAll: keys differ\ngetKeys=%v\nputKeys=%v", getKeys, putKeys)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetAll: entities differ\ngot =%v\nwant=%v", got, want)
	}
	if _, ok := err.(*ErrFieldMismatch); !ok {
		t.Errorf("GetAll: got err=%v, want ErrFieldMismatch", err)
	}
}

func TestKindlessQueries(t *testing.T) {
	type Dee struct {
		I   int
		Why string
	}
	type Dum struct {
		I     int
		Pling string
	}

	c := testutil.Context(ScopeDatastore, ScopeUserEmail)
	parent := NewKey(c, "Tweedle", "tweedle", 0, nil)

	keys := []*Key{
		NewKey(c, "Dee", "dee0", 0, parent),
		NewKey(c, "Dum", "dum1", 0, parent),
		NewKey(c, "Dum", "dum2", 0, parent),
		NewKey(c, "Dum", "dum3", 0, parent),
	}
	src := []interface{}{
		&Dee{1, "binary0001"},
		&Dum{2, "binary0010"},
		&Dum{4, "binary0100"},
		&Dum{8, "binary1000"},
	}
	keys, err := PutMulti(c, keys, src)
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	testCases := []struct {
		desc    string
		query   *Query
		want    []int
		wantErr string
	}{
		{
			desc:  "Dee",
			query: NewQuery("Dee"),
			want:  []int{1},
		},
		{
			desc:  "Doh",
			query: NewQuery("Doh"),
			want:  nil},
		{
			desc:  "Dum",
			query: NewQuery("Dum"),
			want:  []int{2, 4, 8},
		},
		{
			desc:  "",
			query: NewQuery(""),
			want:  []int{1, 2, 4, 8},
		},
		{
			desc:  "Kindless filter",
			query: NewQuery("").Filter("__key__ =", keys[2]),
			want:  []int{4},
		},
		{
			desc:  "Kindless order",
			query: NewQuery("").Order("__key__"),
			want:  []int{1, 2, 4, 8},
		},
		{
			desc:    "Kindless bad filter",
			query:   NewQuery("").Filter("I =", 4),
			wantErr: "kind is required for filter: I",
		},
		{
			desc:    "Kindless bad order",
			query:   NewQuery("").Order("-__key__"),
			wantErr: "kind is required for all orders except __key__ ascending",
		},
	}
loop:
	for _, tc := range testCases {
		q := tc.query.Ancestor(parent)
		gotCount, err := q.Count(c)
		if err != nil {
			if tc.wantErr == "" || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("count %q: err %v, want err %q", tc.desc, err, tc.wantErr)
			}
			continue
		}
		if tc.wantErr != "" {
			t.Errorf("count %q: want err %q", tc.desc, tc.wantErr)
			continue
		}
		if gotCount != len(tc.want) {
			t.Errorf("count %q: got %d want %d", tc.desc, gotCount, len(tc.want))
			continue
		}
		var got []int
		for iter := q.Run(c); ; {
			var dst struct {
				I          int
				Why, Pling string
			}
			_, err := iter.Next(&dst)
			if err == Done {
				break
			}
			if err != nil {
				t.Errorf("iter.Next %q: %v", tc.desc, err)
				continue loop
			}
			got = append(got, dst.I)
		}
		sort.Ints(got)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("elems %q: got %+v want %+v", tc.desc, got, tc.want)
			continue
		}
	}
}
