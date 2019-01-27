// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package ranger

import (
	"fmt"
	"math"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
)

// Range represents a range generated in physical plan building phase.
type Range struct {
	LowVal  []types.Datum
	HighVal []types.Datum

	LowExclude  bool // Low value is exclusive.
	HighExclude bool // High value is exclusive.
}

// Clone clones a Range.
func (ran *Range) Clone() *Range {
	newRange := &Range{
		LowVal:      make([]types.Datum, 0, len(ran.LowVal)),
		HighVal:     make([]types.Datum, 0, len(ran.HighVal)),
		LowExclude:  ran.LowExclude,
		HighExclude: ran.HighExclude,
	}
	for i, length := 0, len(ran.LowVal); i < length; i++ {
		newRange.LowVal = append(newRange.LowVal, ran.LowVal[i])
	}
	for i, length := 0, len(ran.HighVal); i < length; i++ {
		newRange.HighVal = append(newRange.HighVal, ran.HighVal[i])
	}
	return newRange
}

// IsPoint returns if the range is a point.
func (ran *Range) IsPoint(sc *stmtctx.StatementContext) bool {
	if len(ran.LowVal) != len(ran.HighVal) {
		return false
	}
	for i := range ran.LowVal {
		a := ran.LowVal[i]
		b := ran.HighVal[i]
		if a.Kind() == types.KindMinNotNull || b.Kind() == types.KindMaxValue {
			return false
		}
		cmp, err := a.CompareDatum(sc, &b)
		if err != nil {
			return false
		}
		if cmp != 0 {
			return false
		}

		if a.IsNull() {
			return false
		}
	}
	return !ran.LowExclude && !ran.HighExclude
}

// String implements the Stringer interface.
func (ran *Range) String() string {
	lowStrs := make([]string, 0, len(ran.LowVal))
	for _, d := range ran.LowVal {
		lowStrs = append(lowStrs, formatDatum(d, true))
	}
	highStrs := make([]string, 0, len(ran.LowVal))
	for _, d := range ran.HighVal {
		highStrs = append(highStrs, formatDatum(d, false))
	}
	l, r := "[", "]"
	if ran.LowExclude {
		l = "("
	}
	if ran.HighExclude {
		r = ")"
	}
	return l + strings.Join(lowStrs, " ") + "," + strings.Join(highStrs, " ") + r
}

// PrefixEqualLen tells you how long the prefix of the range is a point.
// e.g. If this range is (1 2 3, 1 2 +inf), then the return value is 2.
func (ran *Range) PrefixEqualLen(sc *stmtctx.StatementContext) (int, error) {
	// Here, len(ran.LowVal) always equal to len(ran.HighVal)
	for i := 0; i < len(ran.LowVal); i++ {
		cmp, err := ran.LowVal[i].CompareDatum(sc, &ran.HighVal[i])
		if err != nil {
			return 0, errors.Trace(err)
		}
		if cmp != 0 {
			return i, nil
		}
	}
	return len(ran.LowVal), nil
}

func formatDatum(d types.Datum, isLeftSide bool) string {
	switch d.Kind() {
	case types.KindNull:
		return "NULL"
	case types.KindMinNotNull:
		return "-inf"
	case types.KindMaxValue:
		return "+inf"
	case types.KindInt64:
		switch d.GetInt64() {
		case math.MinInt64:
			if isLeftSide {
				return "-inf"
			}
		case math.MaxInt64:
			if !isLeftSide {
				return "+inf"
			}
		}
	case types.KindUint64:
		if d.GetUint64() == math.MaxUint64 && !isLeftSide {
			return "+inf"
		}
	case types.KindString, types.KindBytes, types.KindMysqlEnum, types.KindMysqlSet,
		types.KindMysqlJSON, types.KindBinaryLiteral, types.KindMysqlBit:
		return fmt.Sprintf("\"%v\"", d.GetValue())
	}
	return fmt.Sprintf("%v", d.GetValue())
}
