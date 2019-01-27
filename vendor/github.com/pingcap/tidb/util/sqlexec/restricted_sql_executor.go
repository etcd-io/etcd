// Copyright 2015 PingCAP, Inc.
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

package sqlexec

import (
	"github.com/pingcap/parser/ast"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/util/chunk"
	"golang.org/x/net/context"
)

// RestrictedSQLExecutor is an interface provides executing restricted sql statement.
// Why we need this interface?
// When we execute some management statements, we need to operate system tables.
// For example when executing create user statement, we need to check if the user already
// exists in the mysql.User table and insert a new row if not exists. In this case, we need
// a convenience way to manipulate system tables. The most simple way is executing sql statement.
// In order to execute sql statement in stmts package, we add this interface to solve dependence problem.
// And in the same time, we do not want this interface becomes a general way to run sql statement.
// We hope this could be used with some restrictions such as only allowing system tables as target,
// do not allowing recursion call.
// For more information please refer to the comments in session.ExecRestrictedSQL().
// This is implemented in session.go.
type RestrictedSQLExecutor interface {
	// ExecRestrictedSQL run sql statement in ctx with some restriction.
	ExecRestrictedSQL(ctx sessionctx.Context, sql string) ([]chunk.Row, []*ast.ResultField, error)
}

// SQLExecutor is an interface provides executing normal sql statement.
// Why we need this interface? To break circle dependence of packages.
// For example, privilege/privileges package need execute SQL, if it use
// session.Session.Execute, then privilege/privileges and tidb would become a circle.
type SQLExecutor interface {
	Execute(ctx context.Context, sql string) ([]RecordSet, error)
}

// SQLParser is an interface provides parsing sql statement.
// To parse a sql statement, we could run parser.New() to get a parser object, and then run Parse method on it.
// But a session already has a parser bind in it, so we define this interface and use session as its implementation,
// thus avoid allocating new parser. See session.SQLParser for more information.
type SQLParser interface {
	ParseSQL(sql, charset, collation string) ([]ast.StmtNode, error)
}

// Statement is an interface for SQL execution.
// NOTE: all Statement implementations must be safe for
// concurrent using by multiple goroutines.
// If the Exec method requires any Execution domain local data,
// they must be held out of the implementing instance.
type Statement interface {
	// OriginText gets the origin SQL text.
	OriginText() string

	// Exec executes SQL and gets a Recordset.
	Exec(ctx context.Context) (RecordSet, error)

	// IsPrepared returns whether this statement is prepared statement.
	IsPrepared() bool

	// IsReadOnly returns if the statement is read only. For example: SelectStmt without lock.
	IsReadOnly() bool

	// RebuildPlan rebuilds the plan of the statement.
	RebuildPlan() (schemaVersion int64, err error)
}

// RecordSet is an abstract result set interface to help get data from Plan.
type RecordSet interface {
	// Fields gets result fields.
	Fields() []*ast.ResultField

	// Next reads records into chunk.
	Next(ctx context.Context, chk *chunk.Chunk) error

	// NewChunk creates a new chunk with initial capacity.
	NewChunk() *chunk.Chunk

	// Close closes the underlying iterator, call Next after Close will
	// restart the iteration.
	Close() error
}
