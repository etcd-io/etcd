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

package expression

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util/chunk"
)

// ExplainInfo implements the Expression interface.
func (expr *ScalarFunction) ExplainInfo() string {
	var buffer bytes.Buffer
	fmt.Fprintf(&buffer, "%s(", expr.FuncName.L)
	for i, arg := range expr.GetArgs() {
		buffer.WriteString(arg.ExplainInfo())
		if i+1 < len(expr.GetArgs()) {
			buffer.WriteString(", ")
		}
	}
	buffer.WriteString(")")
	return buffer.String()
}

// ExplainInfo implements the Expression interface.
func (expr *Column) ExplainInfo() string {
	return expr.String()
}

// ExplainInfo implements the Expression interface.
func (expr *Constant) ExplainInfo() string {
	dt, err := expr.Eval(chunk.Row{})
	if err != nil {
		return "not recognized const vanue"
	}
	return expr.format(dt)
}

func (expr *Constant) format(dt types.Datum) string {
	switch dt.Kind() {
	case types.KindNull:
		return "NULL"
	case types.KindString, types.KindBytes, types.KindMysqlEnum, types.KindMysqlSet,
		types.KindMysqlJSON, types.KindBinaryLiteral, types.KindMysqlBit:
		return fmt.Sprintf("\"%v\"", dt.GetValue())
	}
	return fmt.Sprintf("%v", dt.GetValue())
}

// ExplainExpressionList generates explain information for a list of expressions.
func ExplainExpressionList(exprs []Expression) []byte {
	buffer := bytes.NewBufferString("")
	for i, expr := range exprs {
		buffer.WriteString(expr.ExplainInfo())
		if i+1 < len(exprs) {
			buffer.WriteString(", ")
		}
	}
	return buffer.Bytes()
}

// SortedExplainExpressionList generates explain information for a list of expressions in order.
// In some scenarios, the expr's order may not be stable when executing multiple times.
// So we add a sort to make its explain result stable.
func SortedExplainExpressionList(exprs []Expression) []byte {
	buffer := bytes.NewBufferString("")
	exprInfos := make([]string, 0, len(exprs))
	for _, expr := range exprs {
		exprInfos = append(exprInfos, expr.ExplainInfo())
	}
	sort.Strings(exprInfos)
	for i, info := range exprInfos {
		buffer.WriteString(info)
		if i+1 < len(exprInfos) {
			buffer.WriteString(", ")
		}
	}
	return buffer.Bytes()
}

// ExplainColumnList generates explain information for a list of columns.
func ExplainColumnList(cols []*Column) []byte {
	buffer := bytes.NewBufferString("")
	for i, col := range cols {
		buffer.WriteString(col.ExplainInfo())
		if i+1 < len(cols) {
			buffer.WriteString(", ")
		}
	}
	return buffer.Bytes()
}
