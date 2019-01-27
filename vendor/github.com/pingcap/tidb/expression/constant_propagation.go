// Copyright 2016 PingCAP, Inc.
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

package expression

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
	"github.com/pingcap/tidb/util/disjointset"
	log "github.com/sirupsen/logrus"
)

// MaxPropagateColsCnt means the max number of columns that can participate propagation.
var MaxPropagateColsCnt = 100

type propagateConstantSolver struct {
	colMapper  map[string]int      // colMapper maps column to its index
	unionSet   *disjointset.IntSet // unionSet stores the relations like col_i = col_j
	eqList     []*Constant         // if eqList[i] != nil, it means col_i = eqList[i]
	columns    []*Column           // columns stores all columns appearing in the conditions
	conditions []Expression
	ctx        sessionctx.Context
}

// propagateConstantEQ propagates expressions like 'column = constant' by substituting the constant for column, the
// procedure repeats multiple times. An example runs as following:
// a = d & b * 2 = c & c = d + 2 & b = 1 & a = 4, we pick eq cond b = 1 and a = 4
// d = 4 & 2 = c & c = d + 2 & b = 1 & a = 4, we propagate b = 1 and a = 4 and pick eq cond c = 2 and d = 4
// d = 4 & 2 = c & false & b = 1 & a = 4, we propagate c = 2 and d = 4, and do constant folding: c = d + 2 will be folded as false.
func (s *propagateConstantSolver) propagateConstantEQ() {
	s.eqList = make([]*Constant, len(s.columns))
	visited := make([]bool, len(s.conditions))
	for i := 0; i < MaxPropagateColsCnt; i++ {
		mapper := s.pickNewEQConds(visited)
		if len(mapper) == 0 {
			return
		}
		cols := make([]*Column, 0, len(mapper))
		cons := make([]Expression, 0, len(mapper))
		for id, con := range mapper {
			cols = append(cols, s.columns[id])
			cons = append(cons, con)
		}
		for i, cond := range s.conditions {
			if !visited[i] {
				s.conditions[i] = ColumnSubstitute(cond, NewSchema(cols...), cons)
			}
		}
	}
}

// propagateColumnEQ propagates expressions like 'column A = column B' by adding extra filters
// 'expression(..., column B, ...)' propagated from 'expression(..., column A, ...)' as long as:
//
//  1. The expression is deterministic
//  2. The expression doesn't have any side effect
//
// e.g. For expression a = b and b = c and c = d and c < 1 , we can get extra a < 1 and b < 1 and d < 1.
// However, for a = b and a < rand(), we cannot propagate a < rand() to b < rand() because rand() is non-deterministic
//
// This propagation may bring redundancies that we need to resolve later, for example:
// for a = b and a < 3 and b < 3, we get new a < 3 and b < 3, which are redundant
// for a = b and a < 3 and 3 > b, we get new b < 3 and 3 > a, which are redundant
// for a = b and a < 3 and b < 4, we get new a < 4 and b < 3 but should expect a < 3 and b < 3
// for a = b and a in (3) and b in (4), we get b in (3) and a in (4) but should expect 'false'
//
// TODO: remove redundancies later
//
// We maintain a unionSet representing the equivalent for every two columns.
func (s *propagateConstantSolver) propagateColumnEQ() {
	visited := make([]bool, len(s.conditions))
	s.unionSet = disjointset.NewIntSet(len(s.columns))
	for i := range s.conditions {
		if fun, ok := s.conditions[i].(*ScalarFunction); ok && fun.FuncName.L == ast.EQ {
			lCol, lOk := fun.GetArgs()[0].(*Column)
			rCol, rOk := fun.GetArgs()[1].(*Column)
			if lOk && rOk {
				lID := s.getColID(lCol)
				rID := s.getColID(rCol)
				s.unionSet.Union(lID, rID)
				visited[i] = true
			}
		}
	}

	condsLen := len(s.conditions)
	for i, coli := range s.columns {
		for j := i + 1; j < len(s.columns); j++ {
			// unionSet doesn't have iterate(), we use a two layer loop to iterate col_i = col_j relation
			if s.unionSet.FindRoot(i) != s.unionSet.FindRoot(j) {
				continue
			}
			colj := s.columns[j]
			for k := 0; k < condsLen; k++ {
				if visited[k] {
					// cond_k has been used to retrieve equality relation
					continue
				}
				cond := s.conditions[k]
				replaced, _, newExpr := s.tryToReplaceCond(coli, colj, cond)
				if replaced {
					s.conditions = append(s.conditions, newExpr)
				}
				replaced, _, newExpr = s.tryToReplaceCond(colj, coli, cond)
				if replaced {
					s.conditions = append(s.conditions, newExpr)
				}
			}
		}
	}
}

// validEqualCond checks if the cond is an expression like [column eq constant].
func (s *propagateConstantSolver) validEqualCond(cond Expression) (*Column, *Constant) {
	if eq, ok := cond.(*ScalarFunction); ok {
		if eq.FuncName.L != ast.EQ {
			return nil, nil
		}
		if col, colOk := eq.GetArgs()[0].(*Column); colOk {
			if con, conOk := eq.GetArgs()[1].(*Constant); conOk {
				return col, con
			}
		}
		if col, colOk := eq.GetArgs()[1].(*Column); colOk {
			if con, conOk := eq.GetArgs()[0].(*Constant); conOk {
				return col, con
			}
		}
	}
	return nil, nil
}

// tryToReplaceCond aims to replace all occurrences of column 'src' and try to replace it with 'tgt' in 'cond'
// It returns
//  bool: if a replacement happened
//  bool: if 'cond' contains non-deterministic expression
//  Expression: the replaced expression, or original 'cond' if the replacement didn't happen
//
// For example:
//  for 'a, b, a < 3', it returns 'true, false, b < 3'
//  for 'a, b, sin(a) + cos(a) = 5', it returns 'true, false, returns sin(b) + cos(b) = 5'
//  for 'a, b, cast(a) < rand()', it returns 'false, true, cast(a) < rand()'
func (s *propagateConstantSolver) tryToReplaceCond(src *Column, tgt *Column, cond Expression) (bool, bool, Expression) {
	sf, ok := cond.(*ScalarFunction)
	if !ok {
		return false, false, cond
	}
	replaced := false
	var args []Expression
	if _, ok := unFoldableFunctions[sf.FuncName.L]; ok {
		return false, true, cond
	}
	if _, ok := inequalFunctions[sf.FuncName.L]; ok {
		return false, true, cond
	}
	for idx, expr := range sf.GetArgs() {
		if src.Equal(nil, expr) {
			replaced = true
			if args == nil {
				args = make([]Expression, len(sf.GetArgs()))
				copy(args, sf.GetArgs())
			}
			args[idx] = tgt
		} else {
			subReplaced, isNonDeterminisitic, subExpr := s.tryToReplaceCond(src, tgt, expr)
			if isNonDeterminisitic {
				return false, true, cond
			} else if subReplaced {
				replaced = true
				if args == nil {
					args = make([]Expression, len(sf.GetArgs()))
					copy(args, sf.GetArgs())
				}
				args[idx] = subExpr
			}
		}
	}
	if replaced {
		return true, false, NewFunctionInternal(s.ctx, sf.FuncName.L, sf.GetType(), args...)
	}
	return false, false, cond
}

func (s *propagateConstantSolver) setConds2ConstFalse() {
	s.conditions = []Expression{&Constant{
		Value:   types.NewDatum(false),
		RetType: types.NewFieldType(mysql.TypeTiny),
	}}
}

// pickNewEQConds tries to pick new equal conds and puts them to retMapper.
func (s *propagateConstantSolver) pickNewEQConds(visited []bool) (retMapper map[int]*Constant) {
	retMapper = make(map[int]*Constant)
	for i, cond := range s.conditions {
		if visited[i] {
			continue
		}
		col, con := s.validEqualCond(cond)
		// Then we check if this CNF item is a false constant. If so, we will set the whole condition to false.
		var ok bool
		if col == nil {
			if con, ok = cond.(*Constant); ok {
				value, err := EvalBool(s.ctx, []Expression{con}, chunk.Row{})
				terror.Log(errors.Trace(err))
				if !value {
					s.setConds2ConstFalse()
					return nil
				}
			}
			continue
		}
		visited[i] = true
		updated, foreverFalse := s.tryToUpdateEQList(col, con)
		if foreverFalse {
			s.setConds2ConstFalse()
			return nil
		}
		if updated {
			retMapper[s.getColID(col)] = con
		}
	}
	return
}

// tryToUpdateEQList tries to update the eqList. When the eqList has store this column with a different constant, like
// a = 1 and a = 2, we set the second return value to false.
func (s *propagateConstantSolver) tryToUpdateEQList(col *Column, con *Constant) (bool, bool) {
	if con.Value.IsNull() {
		return false, true
	}
	id := s.getColID(col)
	oldCon := s.eqList[id]
	if oldCon != nil {
		return false, !oldCon.Equal(s.ctx, con)
	}
	s.eqList[id] = con
	return true, false
}

func (s *propagateConstantSolver) solve(conditions []Expression) []Expression {
	cols := make([]*Column, 0, len(conditions))
	for _, cond := range conditions {
		s.conditions = append(s.conditions, SplitCNFItems(cond)...)
		cols = append(cols, ExtractColumns(cond)...)
	}
	for _, col := range cols {
		s.insertCol(col)
	}
	if len(s.columns) > MaxPropagateColsCnt {
		log.Warnf("[const_propagation]Too many columns in a single CNF: the column count is %d, the max count is %d.", len(s.columns), MaxPropagateColsCnt)
		return conditions
	}
	s.propagateConstantEQ()
	s.propagateColumnEQ()
	for i, cond := range s.conditions {
		if dnf, ok := cond.(*ScalarFunction); ok && dnf.FuncName.L == ast.LogicOr {
			dnfItems := SplitDNFItems(cond)
			for j, item := range dnfItems {
				dnfItems[j] = ComposeCNFCondition(s.ctx, PropagateConstant(s.ctx, []Expression{item})...)
			}
			s.conditions[i] = ComposeDNFCondition(s.ctx, dnfItems...)
		}
	}
	return s.conditions
}

func (s *propagateConstantSolver) getColID(col *Column) int {
	code := col.HashCode(nil)
	return s.colMapper[string(code)]
}

func (s *propagateConstantSolver) insertCol(col *Column) {
	code := col.HashCode(nil)
	_, ok := s.colMapper[string(code)]
	if !ok {
		s.colMapper[string(code)] = len(s.colMapper)
		s.columns = append(s.columns, col)
	}
}

// PropagateConstant propagate constant values of deterministic predicates in a condition.
func PropagateConstant(ctx sessionctx.Context, conditions []Expression) []Expression {
	solver := &propagateConstantSolver{
		colMapper: make(map[string]int),
		ctx:       ctx,
	}
	return solver.solve(conditions)
}
