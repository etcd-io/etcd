// Copyright 2016 The etcd Authors
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

/*
The wildcard package provides support for converting
single keys ending in the '*' star wildcard character into
prefix ranges. Backslash escaped '\*' stars are ignored.
*/
package wildcard

// ExpandWildcardToRange enables get '*' to return all
// keys within the range specified by a prefix.
//
// Example: get '/home/user/*' will return all keys with
// the prefix '/home/user/'.
//
// Returns true if conversion/expansion took place.
//
// If removeWild is true, then we will remove the wildcard byte
// before creating the range.
// Backslash escaped (e.g. `\*`) wildcards will not be changed.
//
func ExpandWildcardToRange(r BegEndRange, removeWild bool) bool {
	beg := r.GetBeg()
	end := r.GetEnd()
	lenkey := len(beg)
	lenend := len(end)
	if lenkey == 0 || lenend > 0 {
		return false
	}
	if beg[lenkey-1] != '*' {
		return false
	}
	if lenkey == 1 {
		// request for all keys
		beg = []byte{0}
		end = []byte{0}
		r.SetBeg(beg)
		r.SetEnd(end)
		return true
	}
	if beg[lenkey-2] == '\\' {
		// the star was escaped with a backslash: \*
		beg = beg[:lenkey-1]
		beg[lenkey-2] = '*'
		r.SetBeg(beg)
		return false
	}

	// we have a wildcard query, with keylen >= 2.
	// Fix the begin and end:
	if removeWild {
		beg = beg[:lenkey-1] // remove the '*'
	}
	// setting RangeEnd one bit higher makes a prefix query
	end = make([]byte, lenkey-1)
	copy(end, beg)
	end[lenkey-2]++
	// check for overflow
	if end[lenkey-2] == 0 {
		// yep, overflowed.
		end = append(end, 1)
	}
	r.SetBeg(beg)
	r.SetEnd(end)
	return true
}

// BegEndRange unifies rangePerm, pb.RangeRequest,
// and pb.DeleteRangeRequest
type BegEndRange interface {
	GetBeg() []byte
	SetBeg(beg []byte)

	GetEnd() []byte
	SetEnd(end []byte)
}
