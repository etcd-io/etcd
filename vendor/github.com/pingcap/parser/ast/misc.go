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

package ast

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/auth"
	. "github.com/pingcap/parser/format"
	"github.com/pingcap/parser/model"
	"github.com/pingcap/parser/mysql"
)

var (
	_ StmtNode = &AdminStmt{}
	_ StmtNode = &AlterUserStmt{}
	_ StmtNode = &BeginStmt{}
	_ StmtNode = &BinlogStmt{}
	_ StmtNode = &CommitStmt{}
	_ StmtNode = &CreateUserStmt{}
	_ StmtNode = &DeallocateStmt{}
	_ StmtNode = &DoStmt{}
	_ StmtNode = &ExecuteStmt{}
	_ StmtNode = &ExplainStmt{}
	_ StmtNode = &GrantStmt{}
	_ StmtNode = &PrepareStmt{}
	_ StmtNode = &RollbackStmt{}
	_ StmtNode = &SetPwdStmt{}
	_ StmtNode = &SetStmt{}
	_ StmtNode = &UseStmt{}
	_ StmtNode = &FlushStmt{}
	_ StmtNode = &KillStmt{}
	_ StmtNode = &CreateBindingStmt{}
	_ StmtNode = &DropBindingStmt{}

	_ Node = &PrivElem{}
	_ Node = &VariableAssignment{}
)

// Isolation level constants.
const (
	ReadCommitted   = "READ-COMMITTED"
	ReadUncommitted = "READ-UNCOMMITTED"
	Serializable    = "SERIALIZABLE"
	RepeatableRead  = "REPEATABLE-READ"

	// Valid formats for explain statement.
	ExplainFormatROW = "row"
	ExplainFormatDOT = "dot"
)

var (
	// ExplainFormats stores the valid formats for explain statement, used by validator.
	ExplainFormats = []string{
		ExplainFormatROW,
		ExplainFormatDOT,
	}
)

// TypeOpt is used for parsing data type option from SQL.
type TypeOpt struct {
	IsUnsigned bool
	IsZerofill bool
}

// FloatOpt is used for parsing floating-point type option from SQL.
// See http://dev.mysql.com/doc/refman/5.7/en/floating-point-types.html
type FloatOpt struct {
	Flen    int
	Decimal int
}

// AuthOption is used for parsing create use statement.
type AuthOption struct {
	// ByAuthString set as true, if AuthString is used for authorization. Otherwise, authorization is done by HashString.
	ByAuthString bool
	AuthString   string
	HashString   string
	// TODO: support auth_plugin
}

// TraceStmt is a statement to trace what sql actually does at background.
type TraceStmt struct {
	stmtNode

	Stmt   StmtNode
	Format string
}

// Restore implements Node interface.
func (n *TraceStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *TraceStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*TraceStmt)
	node, ok := n.Stmt.Accept(v)
	if !ok {
		return n, false
	}
	n.Stmt = node.(DMLNode)
	return v.Leave(n)
}

// ExplainStmt is a statement to provide information about how is SQL statement executed
// or get columns information in a table.
// See https://dev.mysql.com/doc/refman/5.7/en/explain.html
type ExplainStmt struct {
	stmtNode

	Stmt    StmtNode
	Format  string
	Analyze bool
}

// Restore implements Node interface.
func (n *ExplainStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *ExplainStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*ExplainStmt)
	node, ok := n.Stmt.Accept(v)
	if !ok {
		return n, false
	}
	n.Stmt = node.(DMLNode)
	return v.Leave(n)
}

// PrepareStmt is a statement to prepares a SQL statement which contains placeholders,
// and it is executed with ExecuteStmt and released with DeallocateStmt.
// See https://dev.mysql.com/doc/refman/5.7/en/prepare.html
type PrepareStmt struct {
	stmtNode

	Name    string
	SQLText string
	SQLVar  *VariableExpr
}

// Restore implements Node interface.
func (n *PrepareStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *PrepareStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*PrepareStmt)
	if n.SQLVar != nil {
		node, ok := n.SQLVar.Accept(v)
		if !ok {
			return n, false
		}
		n.SQLVar = node.(*VariableExpr)
	}
	return v.Leave(n)
}

// DeallocateStmt is a statement to release PreparedStmt.
// See https://dev.mysql.com/doc/refman/5.7/en/deallocate-prepare.html
type DeallocateStmt struct {
	stmtNode

	Name string
}

// Restore implements Node interface.
func (n *DeallocateStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *DeallocateStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*DeallocateStmt)
	return v.Leave(n)
}

// Prepared represents a prepared statement.
type Prepared struct {
	Stmt          StmtNode
	Params        []ParamMarkerExpr
	SchemaVersion int64
	UseCache      bool
}

// ExecuteStmt is a statement to execute PreparedStmt.
// See https://dev.mysql.com/doc/refman/5.7/en/execute.html
type ExecuteStmt struct {
	stmtNode

	Name      string
	UsingVars []ExprNode
	ExecID    uint32
}

// Restore implements Node interface.
func (n *ExecuteStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *ExecuteStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*ExecuteStmt)
	for i, val := range n.UsingVars {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.UsingVars[i] = node.(ExprNode)
	}
	return v.Leave(n)
}

// BeginStmt is a statement to start a new transaction.
// See https://dev.mysql.com/doc/refman/5.7/en/commit.html
type BeginStmt struct {
	stmtNode
}

// Restore implements Node interface.
func (n *BeginStmt) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord("START TRANSACTION")
	return nil
}

// Accept implements Node Accept interface.
func (n *BeginStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*BeginStmt)
	return v.Leave(n)
}

// BinlogStmt is an internal-use statement.
// We just parse and ignore it.
// See http://dev.mysql.com/doc/refman/5.7/en/binlog.html
type BinlogStmt struct {
	stmtNode
	Str string
}

// Restore implements Node interface.
func (n *BinlogStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *BinlogStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*BinlogStmt)
	return v.Leave(n)
}

// CommitStmt is a statement to commit the current transaction.
// See https://dev.mysql.com/doc/refman/5.7/en/commit.html
type CommitStmt struct {
	stmtNode
}

// Restore implements Node interface.
func (n *CommitStmt) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord("COMMIT")
	return nil
}

// Accept implements Node Accept interface.
func (n *CommitStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*CommitStmt)
	return v.Leave(n)
}

// RollbackStmt is a statement to roll back the current transaction.
// See https://dev.mysql.com/doc/refman/5.7/en/commit.html
type RollbackStmt struct {
	stmtNode
}

// Restore implements Node interface.
func (n *RollbackStmt) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord("ROLLBACK")
	return nil
}

// Accept implements Node Accept interface.
func (n *RollbackStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*RollbackStmt)
	return v.Leave(n)
}

// UseStmt is a statement to use the DBName database as the current database.
// See https://dev.mysql.com/doc/refman/5.7/en/use.html
type UseStmt struct {
	stmtNode

	DBName string
}

// Restore implements Node interface.
func (n *UseStmt) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord("USE ")
	ctx.WriteName(n.DBName)
	return nil
}

// Accept implements Node Accept interface.
func (n *UseStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*UseStmt)
	return v.Leave(n)
}

const (
	// SetNames is the const for set names/charset stmt.
	// If VariableAssignment.Name == Names, it should be set names/charset stmt.
	SetNames = "SetNAMES"
)

// VariableAssignment is a variable assignment struct.
type VariableAssignment struct {
	node
	Name     string
	Value    ExprNode
	IsGlobal bool
	IsSystem bool

	// ExtendValue is a way to store extended info.
	// VariableAssignment should be able to store information for SetCharset/SetPWD Stmt.
	// For SetCharsetStmt, Value is charset, ExtendValue is collation.
	// TODO: Use SetStmt to implement set password statement.
	ExtendValue ValueExpr
}

// Restore implements Node interface.
func (n *VariableAssignment) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node interface.
func (n *VariableAssignment) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*VariableAssignment)
	node, ok := n.Value.Accept(v)
	if !ok {
		return n, false
	}
	n.Value = node.(ExprNode)
	return v.Leave(n)
}

// FlushStmtType is the type for FLUSH statement.
type FlushStmtType int

// Flush statement types.
const (
	FlushNone FlushStmtType = iota
	FlushTables
	FlushPrivileges
	FlushStatus
)

// FlushStmt is a statement to flush tables/privileges/optimizer costs and so on.
type FlushStmt struct {
	stmtNode

	Tp              FlushStmtType // Privileges/Tables/...
	NoWriteToBinLog bool
	Tables          []*TableName // For FlushTableStmt, if Tables is empty, it means flush all tables.
	ReadLock        bool
}

// Restore implements Node interface.
func (n *FlushStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *FlushStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*FlushStmt)
	return v.Leave(n)
}

// KillStmt is a statement to kill a query or connection.
type KillStmt struct {
	stmtNode

	// Query indicates whether terminate a single query on this connection or the whole connection.
	// If Query is true, terminates the statement the connection is currently executing, but leaves the connection itself intact.
	// If Query is false, terminates the connection associated with the given ConnectionID, after terminating any statement the connection is executing.
	Query        bool
	ConnectionID uint64
	// TiDBExtension is used to indicate whether the user knows he is sending kill statement to the right tidb-server.
	// When the SQL grammar is "KILL TIDB [CONNECTION | QUERY] connectionID", TiDBExtension will be set.
	// It's a special grammar extension in TiDB. This extension exists because, when the connection is:
	// client -> LVS proxy -> TiDB, and type Ctrl+C in client, the following action will be executed:
	// new a connection; kill xxx;
	// kill command may send to the wrong TiDB, because the exists of LVS proxy, and kill the wrong session.
	// So, "KILL TIDB" grammar is introduced, and it REQUIRES DIRECT client -> TiDB TOPOLOGY.
	// TODO: The standard KILL grammar will be supported once we have global connectionID.
	TiDBExtension bool
}

// Restore implements Node interface.
func (n *KillStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *KillStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*KillStmt)
	return v.Leave(n)
}

// SetStmt is the statement to set variables.
type SetStmt struct {
	stmtNode
	// Variables is the list of variable assignment.
	Variables []*VariableAssignment
}

// Restore implements Node interface.
func (n *SetStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *SetStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*SetStmt)
	for i, val := range n.Variables {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Variables[i] = node.(*VariableAssignment)
	}
	return v.Leave(n)
}

/*
// SetCharsetStmt is a statement to assign values to character and collation variables.
// See https://dev.mysql.com/doc/refman/5.7/en/set-statement.html
type SetCharsetStmt struct {
	stmtNode

	Charset string
	Collate string
}

// Accept implements Node Accept interface.
func (n *SetCharsetStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*SetCharsetStmt)
	return v.Leave(n)
}
*/

// SetPwdStmt is a statement to assign a password to user account.
// See https://dev.mysql.com/doc/refman/5.7/en/set-password.html
type SetPwdStmt struct {
	stmtNode

	User     *auth.UserIdentity
	Password string
}

// Restore implements Node interface.
func (n *SetPwdStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// SecureText implements SensitiveStatement interface.
func (n *SetPwdStmt) SecureText() string {
	return fmt.Sprintf("set password for user %s", n.User)
}

// Accept implements Node Accept interface.
func (n *SetPwdStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*SetPwdStmt)
	return v.Leave(n)
}

// UserSpec is used for parsing create user statement.
type UserSpec struct {
	User    *auth.UserIdentity
	AuthOpt *AuthOption
}

// SecurityString formats the UserSpec without password information.
func (u *UserSpec) SecurityString() string {
	withPassword := false
	if opt := u.AuthOpt; opt != nil {
		if len(opt.AuthString) > 0 || len(opt.HashString) > 0 {
			withPassword = true
		}
	}
	if withPassword {
		return fmt.Sprintf("{%s password = ***}", u.User)
	}
	return u.User.String()
}

// EncodedPassword returns the encoded password (which is the real data mysql.user).
// The boolean value indicates input's password format is legal or not.
func (u *UserSpec) EncodedPassword() (string, bool) {
	if u.AuthOpt == nil {
		return "", true
	}

	opt := u.AuthOpt
	if opt.ByAuthString {
		return auth.EncodePassword(opt.AuthString), true
	}

	// Not a legal password string.
	if len(opt.HashString) != 41 || !strings.HasPrefix(opt.HashString, "*") {
		return "", false
	}
	return opt.HashString, true
}

// CreateUserStmt creates user account.
// See https://dev.mysql.com/doc/refman/5.7/en/create-user.html
type CreateUserStmt struct {
	stmtNode

	IfNotExists bool
	Specs       []*UserSpec
}

// Restore implements Node interface.
func (n *CreateUserStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *CreateUserStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*CreateUserStmt)
	return v.Leave(n)
}

// SecureText implements SensitiveStatement interface.
func (n *CreateUserStmt) SecureText() string {
	var buf bytes.Buffer
	buf.WriteString("create user")
	for _, user := range n.Specs {
		buf.WriteString(" ")
		buf.WriteString(user.SecurityString())
	}
	return buf.String()
}

// AlterUserStmt modifies user account.
// See https://dev.mysql.com/doc/refman/5.7/en/alter-user.html
type AlterUserStmt struct {
	stmtNode

	IfExists    bool
	CurrentAuth *AuthOption
	Specs       []*UserSpec
}

// Restore implements Node interface.
func (n *AlterUserStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// SecureText implements SensitiveStatement interface.
func (n *AlterUserStmt) SecureText() string {
	var buf bytes.Buffer
	buf.WriteString("alter user")
	for _, user := range n.Specs {
		buf.WriteString(" ")
		buf.WriteString(user.SecurityString())
	}
	return buf.String()
}

// Accept implements Node Accept interface.
func (n *AlterUserStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*AlterUserStmt)
	return v.Leave(n)
}

// DropUserStmt creates user account.
// See http://dev.mysql.com/doc/refman/5.7/en/drop-user.html
type DropUserStmt struct {
	stmtNode

	IfExists bool
	UserList []*auth.UserIdentity
}

// Restore implements Node interface.
func (n *DropUserStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *DropUserStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*DropUserStmt)
	return v.Leave(n)
}

// CreateBindingStmt creates sql binding hint.
type CreateBindingStmt struct {
	stmtNode

	GlobalScope bool
	OriginSel   StmtNode
	HintedSel   StmtNode
}

func (n *CreateBindingStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

func (n *CreateBindingStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*CreateBindingStmt)
	selnode, ok := n.OriginSel.Accept(v)
	if !ok {
		return n, false
	}
	n.OriginSel = selnode.(*SelectStmt)
	hintedSelnode, ok := n.HintedSel.Accept(v)
	if !ok {
		return n, false
	}
	n.HintedSel = hintedSelnode.(*SelectStmt)
	return v.Leave(n)
}

// DropBindingStmt deletes sql binding hint.
type DropBindingStmt struct {
	stmtNode

	GlobalScope bool
	OriginSel   StmtNode
}

func (n *DropBindingStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

func (n *DropBindingStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*DropBindingStmt)
	selnode, ok := n.OriginSel.Accept(v)
	if !ok {
		return n, false
	}
	n.OriginSel = selnode.(*SelectStmt)
	return v.Leave(n)
}

// DoStmt is the struct for DO statement.
type DoStmt struct {
	stmtNode

	Exprs []ExprNode
}

// Restore implements Node interface.
func (n *DoStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *DoStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*DoStmt)
	for i, val := range n.Exprs {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Exprs[i] = node.(ExprNode)
	}
	return v.Leave(n)
}

// AdminStmtType is the type for admin statement.
type AdminStmtType int

// Admin statement types.
const (
	AdminShowDDL = iota + 1
	AdminCheckTable
	AdminShowDDLJobs
	AdminCancelDDLJobs
	AdminCheckIndex
	AdminRecoverIndex
	AdminCleanupIndex
	AdminCheckIndexRange
	AdminShowDDLJobQueries
	AdminChecksumTable
	AdminShowSlow
	AdminShowNextRowID
	AdminRestoreTable
)

// HandleRange represents a range where handle value >= Begin and < End.
type HandleRange struct {
	Begin int64
	End   int64
}

// ShowSlowType defines the type for SlowSlow statement.
type ShowSlowType int

const (
	// ShowSlowTop is a ShowSlowType constant.
	ShowSlowTop ShowSlowType = iota
	// ShowSlowRecent is a ShowSlowType constant.
	ShowSlowRecent
)

// ShowSlowKind defines the kind for SlowSlow statement when the type is ShowSlowTop.
type ShowSlowKind int

const (
	// ShowSlowKindDefault is a ShowSlowKind constant.
	ShowSlowKindDefault ShowSlowKind = iota
	// ShowSlowKindInternal is a ShowSlowKind constant.
	ShowSlowKindInternal
	// ShowSlowKindAll is a ShowSlowKind constant.
	ShowSlowKindAll
)

// ShowSlow is used for the following command:
//	admin show slow top [ internal | all] N
//	admin show slow recent N
type ShowSlow struct {
	Tp    ShowSlowType
	Count uint64
	Kind  ShowSlowKind
}

// AdminStmt is the struct for Admin statement.
type AdminStmt struct {
	stmtNode

	Tp        AdminStmtType
	Index     string
	Tables    []*TableName
	JobIDs    []int64
	JobNumber int64

	HandleRanges []HandleRange
	ShowSlow     *ShowSlow
}

// Restore implements Node interface.
func (n *AdminStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *AdminStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}

	n = newNode.(*AdminStmt)
	for i, val := range n.Tables {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Tables[i] = node.(*TableName)
	}

	return v.Leave(n)
}

// PrivElem is the privilege type and optional column list.
type PrivElem struct {
	node

	Priv mysql.PrivilegeType
	Cols []*ColumnName
}

// Restore implements Node interface.
func (n *PrivElem) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *PrivElem) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*PrivElem)
	for i, val := range n.Cols {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Cols[i] = node.(*ColumnName)
	}
	return v.Leave(n)
}

// ObjectTypeType is the type for object type.
type ObjectTypeType int

const (
	// ObjectTypeNone is for empty object type.
	ObjectTypeNone ObjectTypeType = iota + 1
	// ObjectTypeTable means the following object is a table.
	ObjectTypeTable
)

// GrantLevelType is the type for grant level.
type GrantLevelType int

const (
	// GrantLevelNone is the dummy const for default value.
	GrantLevelNone GrantLevelType = iota + 1
	// GrantLevelGlobal means the privileges are administrative or apply to all databases on a given server.
	GrantLevelGlobal
	// GrantLevelDB means the privileges apply to all objects in a given database.
	GrantLevelDB
	// GrantLevelTable means the privileges apply to all columns in a given table.
	GrantLevelTable
)

// GrantLevel is used for store the privilege scope.
type GrantLevel struct {
	Level     GrantLevelType
	DBName    string
	TableName string
}

// RevokeStmt is the struct for REVOKE statement.
type RevokeStmt struct {
	stmtNode

	Privs      []*PrivElem
	ObjectType ObjectTypeType
	Level      *GrantLevel
	Users      []*UserSpec
}

// Restore implements Node interface.
func (n *RevokeStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// Accept implements Node Accept interface.
func (n *RevokeStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*RevokeStmt)
	for i, val := range n.Privs {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Privs[i] = node.(*PrivElem)
	}
	return v.Leave(n)
}

// GrantStmt is the struct for GRANT statement.
type GrantStmt struct {
	stmtNode

	Privs      []*PrivElem
	ObjectType ObjectTypeType
	Level      *GrantLevel
	Users      []*UserSpec
	WithGrant  bool
}

// Restore implements Node interface.
func (n *GrantStmt) Restore(ctx *RestoreCtx) error {
	return errors.New("Not implemented")
}

// SecureText implements SensitiveStatement interface.
func (n *GrantStmt) SecureText() string {
	text := n.text
	// Filter "identified by xxx" because it would expose password information.
	idx := strings.Index(strings.ToLower(text), "identified")
	if idx > 0 {
		text = text[:idx]
	}
	return text
}

// Accept implements Node Accept interface.
func (n *GrantStmt) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*GrantStmt)
	for i, val := range n.Privs {
		node, ok := val.Accept(v)
		if !ok {
			return n, false
		}
		n.Privs[i] = node.(*PrivElem)
	}
	return v.Leave(n)
}

// Ident is the table identifier composed of schema name and table name.
type Ident struct {
	Schema model.CIStr
	Name   model.CIStr
}

// String implements fmt.Stringer interface.
func (i Ident) String() string {
	if i.Schema.O == "" {
		return i.Name.O
	}
	return fmt.Sprintf("%s.%s", i.Schema, i.Name)
}

// SelectStmtOpts wrap around select hints and switches
type SelectStmtOpts struct {
	Distinct      bool
	SQLCache      bool
	CalcFoundRows bool
	StraightJoin  bool
	Priority      mysql.PriorityEnum
	TableHints    []*TableOptimizerHint
}

// TableOptimizerHint is Table level optimizer hint
type TableOptimizerHint struct {
	node
	// HintName is the name or alias of the table(s) which the hint will affect.
	// Table hints has no schema info
	// It allows only table name or alias (if table has an alias)
	HintName model.CIStr
	Tables   []model.CIStr
	// Statement Execution Time Optimizer Hints
	// See https://dev.mysql.com/doc/refman/5.7/en/optimizer-hints.html#optimizer-hints-execution-time
	MaxExecutionTime uint64
}

// Restore implements Node interface.
func (n *TableOptimizerHint) Restore(ctx *RestoreCtx) error {
	ctx.WriteKeyWord(n.HintName.String())
	ctx.WritePlain("(")
	if n.HintName.L == "max_execution_time" {
		ctx.WritePlainf("%d", n.MaxExecutionTime)
	} else {
		for i, table := range n.Tables {
			if i != 0 {
				ctx.WritePlain(", ")
			}
			ctx.WriteName(table.String())
		}
	}
	ctx.WritePlain(")")
	return nil
}

// Accept implements Node Accept interface.
func (n *TableOptimizerHint) Accept(v Visitor) (Node, bool) {
	newNode, skipChildren := v.Enter(n)
	if skipChildren {
		return v.Leave(newNode)
	}
	n = newNode.(*TableOptimizerHint)
	return v.Leave(n)
}

// NewDecimal creates a types.Decimal value, it's provided by parser driver.
var NewDecimal func(string) (interface{}, error)

// NewHexLiteral creates a types.HexLiteral value, it's provided by parser driver.
var NewHexLiteral func(string) (interface{}, error)

// NewBitLiteral creates a types.BitLiteral value, it's provided by parser driver.
var NewBitLiteral func(string) (interface{}, error)
