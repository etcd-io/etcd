// Copyright 2025 The etcd Authors
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

package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	keyHeader  = "header"
	keyMember  = "member"
	keyMembers = "members"

	keyClusterID = "cluster_id"
	keyMemberID  = "member_id"
	keyRaftTerm  = "raft_term"
	keyRevision  = "revision"
	keyID        = "ID"
)

func assertNumericFieldEqual(t *testing.T, obj map[string]any, key string, want int64) {
	raw, ok := obj[key]
	require.Truef(t, ok, "missing key %q in map %v", key, obj)

	n, ok := raw.(json.Number)
	require.Truef(t, ok, "field %q is not json.Number: %v", key, raw)

	val, err := n.Int64()
	require.NoErrorf(t, err, "failed to convert field %q to int64: %v", key, n)

	assert.Equalf(t, want, val, "unexpected value for field %q", key)
}

func assertHexFieldEqual(t *testing.T, obj map[string]any, key string, want string) {
	raw, ok := obj[key]
	require.Truef(t, ok, "missing key %q in map %v", key, obj)

	str, ok := raw.(string)
	require.Truef(t, ok, "field %q is not a string: %v", key, str)

	assert.Equalf(t, want, str, "unexpected value for hex field %q", key)
}

func assertHeader(t *testing.T, testGroup *testScenario, tt *testCase, got map[string]any) {
	rawHeader, ok := got[keyHeader]
	require.Truef(t, ok, "output does not contain %q field: %v", keyHeader, got)
	header, ok := rawHeader.(map[string]any)
	require.Truef(t, ok, "field %q is not map[string]any: %v", keyHeader, rawHeader)

	if testGroup.isHex {
		assertHexFieldEqual(t, header, keyClusterID, tt.wantHexString)
		assertHexFieldEqual(t, header, keyMemberID, tt.wantHexString)
	} else {
		assertNumericFieldEqual(t, header, keyClusterID, tt.wantDecimalNumber)
		assertNumericFieldEqual(t, header, keyMemberID, tt.wantDecimalNumber)
	}
	assertNumericFieldEqual(t, header, keyRaftTerm, tt.wantDecimalNumber)
	assertNumericFieldEqual(t, header, keyRevision, tt.wantDecimalNumber)
}

func assertMember(t *testing.T, testGroup *testScenario, tt *testCase, rawMember any) {
	member, ok := rawMember.(map[string]any)
	require.Truef(t, ok, "field %q is not map[string]any: %v", keyMember, rawMember)

	if testGroup.isHex {
		assertHexFieldEqual(t, member, keyID, tt.wantHexString)
	} else {
		assertNumericFieldEqual(t, member, keyID, tt.wantDecimalNumber)
	}
}

func assertMembers(t *testing.T, testGroup *testScenario, tt *testCase, got map[string]any) {
	rawMembers, ok := got[keyMembers]
	require.Truef(t, ok, "output does not contain %q field: %v", keyMembers, got)
	members, ok := rawMembers.([]any)
	require.Truef(t, ok, "field %q is not []any: %v", keyMembers, rawMembers)

	for _, rawMember := range members {
		assertMember(t, testGroup, tt, rawMember)
	}
}

type testCase struct {
	number            uint64
	wantHexString     string
	wantDecimalNumber int64
}

type testScenario struct {
	name  string
	isHex bool
	cases []testCase
}

var testCases = []testCase{
	{1, "1", 1},
	{100, "64", 100},
	{1234567890, "499602d2", 1234567890},
	{math.MaxInt64, "7fffffffffffffff", math.MaxInt64},
}

func TestMemberAdd(t *testing.T) {
	tests := []testScenario{
		{name: "decimal", isHex: false, cases: testCases},
		{name: "hex", isHex: true, cases: testCases},
	}

	for _, testGroup := range tests {
		t.Run(testGroup.name, func(t *testing.T) {
			var buffer bytes.Buffer
			p := &jsonPrinter{writer: &buffer, isHex: testGroup.isHex}

			for _, tt := range testGroup.cases {
				t.Run(fmt.Sprintf("number=%d", tt.number), func(t *testing.T) {
					buffer.Reset()
					decoder := json.NewDecoder(&buffer)
					decoder.UseNumber()

					response := clientv3.MemberAddResponse{
						Header: &pb.ResponseHeader{
							ClusterId: tt.number,
							MemberId:  tt.number,
							Revision:  int64(tt.number),
							RaftTerm:  tt.number,
						},
						Member:  &pb.Member{ID: tt.number},
						Members: []*pb.Member{{ID: tt.number}},
					}
					p.MemberAdd(response)

					var got map[string]any
					err := decoder.Decode(&got)
					require.NoErrorf(t, err, "failed to decode JSON")

					assertHeader(t, &testGroup, &tt, got)

					rawMember, ok := got[keyMember]
					require.Truef(t, ok, "output does not contain %q field: %v", keyMember, got)
					assertMember(t, &testGroup, &tt, rawMember)

					assertMembers(t, &testGroup, &tt, got)
				})
			}
		})
	}
}

func TestMemberRemove(t *testing.T) {
	tests := []testScenario{
		{name: "decimal", isHex: false, cases: testCases},
		{name: "hex", isHex: true, cases: testCases},
	}

	for _, testGroup := range tests {
		t.Run(testGroup.name, func(t *testing.T) {
			var buffer bytes.Buffer
			p := &jsonPrinter{writer: &buffer, isHex: testGroup.isHex}

			for _, tt := range testGroup.cases {
				t.Run(fmt.Sprintf("number=%d", tt.number), func(t *testing.T) {
					buffer.Reset()
					decoder := json.NewDecoder(&buffer)
					decoder.UseNumber()

					response := clientv3.MemberRemoveResponse{
						Header: &pb.ResponseHeader{
							ClusterId: tt.number,
							MemberId:  tt.number,
							Revision:  int64(tt.number),
							RaftTerm:  tt.number,
						},
						Members: []*pb.Member{{ID: tt.number}},
					}
					p.MemberRemove(0, response)

					var got map[string]any
					err := decoder.Decode(&got)
					require.NoErrorf(t, err, "failed to decode JSON")

					assertHeader(t, &testGroup, &tt, got)
					assertMembers(t, &testGroup, &tt, got)
				})
			}
		})
	}
}

func TestMemberUpdate(t *testing.T) {
	tests := []testScenario{
		{name: "decimal", isHex: false, cases: testCases},
		{name: "hex", isHex: true, cases: testCases},
	}

	for _, testGroup := range tests {
		t.Run(testGroup.name, func(t *testing.T) {
			var buffer bytes.Buffer
			p := &jsonPrinter{writer: &buffer, isHex: testGroup.isHex}

			for _, tt := range testGroup.cases {
				t.Run(fmt.Sprintf("number=%d", tt.number), func(t *testing.T) {
					buffer.Reset()
					decoder := json.NewDecoder(&buffer)
					decoder.UseNumber()

					response := clientv3.MemberUpdateResponse{
						Header: &pb.ResponseHeader{
							ClusterId: tt.number,
							MemberId:  tt.number,
							Revision:  int64(tt.number),
							RaftTerm:  tt.number,
						},
						Members: []*pb.Member{{ID: tt.number}},
					}
					p.MemberUpdate(0, response)

					var got map[string]any
					err := decoder.Decode(&got)
					require.NoErrorf(t, err, "failed to decode JSON")

					assertHeader(t, &testGroup, &tt, got)
					assertMembers(t, &testGroup, &tt, got)
				})
			}
		})
	}
}

func TestMemberPromote(t *testing.T) {
	tests := []testScenario{
		{name: "decimal", isHex: false, cases: testCases},
		{name: "hex", isHex: true, cases: testCases},
	}

	for _, testGroup := range tests {
		t.Run(testGroup.name, func(t *testing.T) {
			var buffer bytes.Buffer
			p := &jsonPrinter{writer: &buffer, isHex: testGroup.isHex}

			for _, tt := range testGroup.cases {
				t.Run(fmt.Sprintf("number=%d", tt.number), func(t *testing.T) {
					buffer.Reset()
					decoder := json.NewDecoder(&buffer)
					decoder.UseNumber()

					response := clientv3.MemberPromoteResponse{
						Header: &pb.ResponseHeader{
							ClusterId: tt.number,
							MemberId:  tt.number,
							Revision:  int64(tt.number),
							RaftTerm:  tt.number,
						},
						Members: []*pb.Member{{ID: tt.number}},
					}
					p.MemberPromote(0, response)

					var got map[string]any
					err := decoder.Decode(&got)
					require.NoErrorf(t, err, "failed to decode JSON")

					assertHeader(t, &testGroup, &tt, got)
					assertMembers(t, &testGroup, &tt, got)
				})
			}
		})
	}
}

func TestMemberList(t *testing.T) {
	tests := []testScenario{
		{name: "decimal", isHex: false, cases: testCases},
		{name: "hex", isHex: true, cases: testCases},
	}

	for _, testGroup := range tests {
		t.Run(testGroup.name, func(t *testing.T) {
			var buffer bytes.Buffer
			p := &jsonPrinter{writer: &buffer, isHex: testGroup.isHex}

			for _, tt := range testGroup.cases {
				t.Run(fmt.Sprintf("number=%d", tt.number), func(t *testing.T) {
					buffer.Reset()
					decoder := json.NewDecoder(&buffer)
					decoder.UseNumber()

					response := clientv3.MemberListResponse{
						Header: &pb.ResponseHeader{
							ClusterId: tt.number,
							MemberId:  tt.number,
							Revision:  int64(tt.number),
							RaftTerm:  tt.number,
						},
						Members: []*pb.Member{{ID: tt.number}},
					}
					p.MemberList(response)

					var got map[string]any
					err := decoder.Decode(&got)
					require.NoErrorf(t, err, "failed to decode JSON")

					assertHeader(t, &testGroup, &tt, got)
					assertMembers(t, &testGroup, &tt, got)
				})
			}
		})
	}
}
