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

package ast

import "github.com/pingcap/parser/model"

var (
	_ StmtNode = &AnalyzeTableStmt{}
	_ StmtNode = &DropStatsStmt{}
	_ StmtNode = &LoadStatsStmt{}
)

// AnalyzeTableStmt is used to create table statistics.
type AnalyzeTableStmt struct {
	stmtNode

	TableNames     []*TableName
	PartitionNames []model.CIStr
	IndexNames     []model.CIStr
	MaxNumBuckets  uint64

	// IndexFlag is true when we only analyze indices for a table.
	IndexFlag bool
}

// Accept implements Node Accept interface.
func (n *AnalyzeTableStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*AnalyzeTableStmt)
	for i, val := range n.TableNames {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.TableNames[i] = node.(*TableName)
	}
	return v.Leave(n)
}

// DropStatsStmt is used to drop table statistics.
type DropStatsStmt struct {
	stmtNode

	Table *TableName
}

// Accept implements Node Accept interface.
func (n *DropStatsStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*DropStatsStmt)
	node, ok := n.Table.Accept(v)
	if !ok {
		return n, false
	}
	n.Table = node.(*TableName)
	return v.Leave(n)
}

// LoadStatsStmt is the statement node for loading statistic.
type LoadStatsStmt struct {
	stmtNode

	Path string
}

// Accept implements Node Accept interface.
func (n *LoadStatsStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*LoadStatsStmt)
	return v.Leave(n)
}
