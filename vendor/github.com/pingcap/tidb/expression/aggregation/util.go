// Copyright 2018 PingCAP, Inc.
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

package aggregation

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/sessionctx/stmtctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/codec"
	"github.com/pingcap/tidb/util/mvmap"
)

// distinctChecker stores existing keys and checks if given data is distinct.
type distinctChecker struct {
	existingKeys *mvmap.MVMap
	key          []byte
	vals         [][]byte
	sc           *stmtctx.StatementContext
}

// createDistinctChecker creates a new distinct checker.
func createDistinctChecker(sc *stmtctx.StatementContext) *distinctChecker {
	return &distinctChecker{
		existingKeys: mvmap.NewMVMap(),
		sc:           sc,
	}
}

// Check checks if values is distinct.
func (d *distinctChecker) Check(values []types.Datum) (bool, error) {
	d.key = d.key[:0]
	var err error
	d.key, err = codec.EncodeValue(d.sc, d.key, values...)
	if err != nil {
		return false, errors.Trace(err)
	}
	d.vals = d.existingKeys.Get(d.key, d.vals[:0])
	if len(d.vals) > 0 {
		return false, nil
	}
	d.existingKeys.Put(d.key, []byte{})
	return true, nil
}

// calculateSum adds v to sum.
func calculateSum(sc *stmtctx.StatementContext, sum, v types.Datum) (data types.Datum, err error) {
	// for avg and sum calculation
	// avg and sum use decimal for integer and decimal type, use float for others
	// see https://dev.mysql.com/doc/refman/5.7/en/group-by-functions.html

	switch v.Kind() {
	case types.KindNull:
	case types.KindInt64, types.KindUint64:
		var d *types.MyDecimal
		d, err = v.ToDecimal(sc)
		if err == nil {
			data = types.NewDecimalDatum(d)
		}
	case types.KindMysqlDecimal:
		data = types.CopyDatum(v)
	default:
		var f float64
		f, err = v.ToFloat64(sc)
		if err == nil {
			data = types.NewFloat64Datum(f)
		}
	}

	if err != nil {
		return data, errors.Trace(err)
	}
	if data.IsNull() {
		return sum, nil
	}
	switch sum.Kind() {
	case types.KindNull:
		return data, nil
	case types.KindFloat64, types.KindMysqlDecimal:
		return types.ComputePlus(sum, data)
	default:
		return data, errors.Errorf("invalid value %v for aggregate", sum.Kind())
	}
}
