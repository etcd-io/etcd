// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package osv

import (
	"encoding/json"
	"fmt"
)

type ReviewStatus int

const (
	ReviewStatusUnknown ReviewStatus = iota
	ReviewStatusUnreviewed
	ReviewStatusReviewed
)

var statusStrs = []string{
	ReviewStatusUnknown:    "",
	ReviewStatusUnreviewed: "UNREVIEWED",
	ReviewStatusReviewed:   "REVIEWED",
}

func (r ReviewStatus) String() string {
	if !r.IsValid() {
		return fmt.Sprintf("INVALID(%d)", r)
	}
	return statusStrs[r]
}

func ReviewStatusValues() []string {
	return statusStrs[1:]
}

func (r ReviewStatus) IsValid() bool {
	return int(r) >= 0 && int(r) < len(statusStrs)
}

func ToReviewStatus(s string) (ReviewStatus, bool) {
	for stat, str := range statusStrs {
		if s == str {
			return ReviewStatus(stat), true
		}
	}
	return 0, false
}

func (r ReviewStatus) MarshalJSON() ([]byte, error) {
	if !r.IsValid() {
		return nil, fmt.Errorf("MarshalJSON: unrecognized review status: %d", r)
	}
	return json.Marshal(r.String())
}

func (r *ReviewStatus) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if rs, ok := ToReviewStatus(s); ok {
		*r = rs
		return nil
	}
	return fmt.Errorf("UnmarshalJSON: unrecognized review status: %s", s)
}
