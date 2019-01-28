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
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/tidb/expression"
	"github.com/pingcap/tidb/types"
)

// conditionChecker checks if this condition can be pushed to index planner.
type conditionChecker struct {
	colName       model.CIStr
	shouldReserve bool // check if a access condition should be reserved in filter conditions.
	length        int
}

func (c *conditionChecker) check(condition expression.Expression) bool {
	switch x := condition.(type) {
	case *expression.ScalarFunction:
		return c.checkScalarFunction(x)
	case *expression.Column:
		return c.checkColumn(x)
	case *expression.Constant:
		return true
	}
	return false
}

func (c *conditionChecker) checkScalarFunction(scalar *expression.ScalarFunction) bool {
	switch scalar.FuncName.L {
	case ast.LogicOr, ast.LogicAnd:
		return c.check(scalar.GetArgs()[0]) && c.check(scalar.GetArgs()[1])
	case ast.EQ, ast.NE, ast.GE, ast.GT, ast.LE, ast.LT:
		if _, ok := scalar.GetArgs()[0].(*expression.Constant); ok {
			if c.checkColumn(scalar.GetArgs()[1]) {
				return scalar.FuncName.L != ast.NE || c.length == types.UnspecifiedLength
			}
		}
		if _, ok := scalar.GetArgs()[1].(*expression.Constant); ok {
			if c.checkColumn(scalar.GetArgs()[0]) {
				return scalar.FuncName.L != ast.NE || c.length == types.UnspecifiedLength
			}
		}
	case ast.IsNull, ast.IsTruth, ast.IsFalsity:
		return c.checkColumn(scalar.GetArgs()[0])
	case ast.UnaryNot:
		// TODO: support "not like" convert to access conditions.
		if s, ok := scalar.GetArgs()[0].(*expression.ScalarFunction); ok {
			if s.FuncName.L == ast.Like {
				return false
			}
		} else {
			// "not column" or "not constant" can't lead to a range.
			return false
		}
		return c.check(scalar.GetArgs()[0])
	case ast.In:
		if !c.checkColumn(scalar.GetArgs()[0]) {
			return false
		}
		for _, v := range scalar.GetArgs()[1:] {
			if _, ok := v.(*expression.Constant); !ok {
				return false
			}
		}
		return true
	case ast.Like:
		return c.checkLikeFunc(scalar)
	case ast.GetParam:
		return true
	}
	return false
}

func (c *conditionChecker) checkLikeFunc(scalar *expression.ScalarFunction) bool {
	if !c.checkColumn(scalar.GetArgs()[0]) {
		return false
	}
	pattern, ok := scalar.GetArgs()[1].(*expression.Constant)
	if !ok {
		return false

	}
	if pattern.Value.IsNull() {
		return false
	}
	patternStr, err := pattern.Value.ToString()
	if err != nil {
		return false
	}
	if len(patternStr) == 0 {
		return true
	}
	escape := byte(scalar.GetArgs()[2].(*expression.Constant).Value.GetInt64())
	for i := 0; i < len(patternStr); i++ {
		if patternStr[i] == escape {
			i++
			if i < len(patternStr)-1 {
				continue
			}
			break
		}
		if i == 0 && (patternStr[i] == '%' || patternStr[i] == '_') {
			return false
		}
		if patternStr[i] == '%' {
			if i != len(patternStr)-1 {
				c.shouldReserve = true
			}
			break
		}
		if patternStr[i] == '_' {
			c.shouldReserve = true
			break
		}
	}
	return true
}

func (c *conditionChecker) checkColumn(expr expression.Expression) bool {
	col, ok := expr.(*expression.Column)
	if !ok {
		return false
	}
	if c.colName.L != "" {
		return c.colName.L == col.ColName.L
	}
	return true
}
