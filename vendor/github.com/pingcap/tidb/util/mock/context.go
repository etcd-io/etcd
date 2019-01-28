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

// Package mock is just for test only.
package mock

import (
	"fmt"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/owner"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/util"
	"github.com/pingcap/tidb/util/kvcache"
	"github.com/pingcap/tidb/util/sqlexec"
	binlog "github.com/pingcap/tipb/go-binlog"
	"golang.org/x/net/context"
)

var _ sessionctx.Context = (*Context)(nil)
var _ sqlexec.SQLExecutor = (*Context)(nil)

// Context represents mocked sessionctx.Context.
type Context struct {
	values      map[fmt.Stringer]interface{}
	txn         wrapTxn    // mock global variable
	Store       kv.Storage // mock global variable
	sessionVars *variable.SessionVars
	mux         sync.Mutex // fix data race in ddl test.
	ctx         context.Context
	cancel      context.CancelFunc
	sm          util.SessionManager
	pcache      *kvcache.SimpleLRUCache
}

type wrapTxn struct {
	kv.Transaction
}

func (txn *wrapTxn) Valid() bool {
	return txn.Transaction != nil && txn.Transaction.Valid()
}

// Execute implements sqlexec.SQLExecutor Execute interface.
func (c *Context) Execute(ctx context.Context, sql string) ([]sqlexec.RecordSet, error) {
	return nil, errors.Errorf("Not Support.")
}

// DDLOwnerChecker returns owner.DDLOwnerChecker.
func (c *Context) DDLOwnerChecker() owner.DDLOwnerChecker {
	return nil
}

// SetValue implements sessionctx.Context SetValue interface.
func (c *Context) SetValue(key fmt.Stringer, value interface{}) {
	c.values[key] = value
}

// Value implements sessionctx.Context Value interface.
func (c *Context) Value(key fmt.Stringer) interface{} {
	value := c.values[key]
	return value
}

// ClearValue implements sessionctx.Context ClearValue interface.
func (c *Context) ClearValue(key fmt.Stringer) {
	delete(c.values, key)
}

// GetSessionVars implements the sessionctx.Context GetSessionVars interface.
func (c *Context) GetSessionVars() *variable.SessionVars {
	return c.sessionVars
}

// Txn implements sessionctx.Context Txn interface.
func (c *Context) Txn(bool) (kv.Transaction, error) {
	return &c.txn, nil
}

// GetClient implements sessionctx.Context GetClient interface.
func (c *Context) GetClient() kv.Client {
	if c.Store == nil {
		return nil
	}
	return c.Store.GetClient()
}

// GetGlobalSysVar implements GlobalVarAccessor GetGlobalSysVar interface.
func (c *Context) GetGlobalSysVar(ctx sessionctx.Context, name string) (string, error) {
	v := variable.GetSysVar(name)
	if v == nil {
		return "", variable.UnknownSystemVar.GenWithStackByArgs(name)
	}
	return v.Value, nil
}

// SetGlobalSysVar implements GlobalVarAccessor SetGlobalSysVar interface.
func (c *Context) SetGlobalSysVar(ctx sessionctx.Context, name string, value string) error {
	v := variable.GetSysVar(name)
	if v == nil {
		return variable.UnknownSystemVar.GenWithStackByArgs(name)
	}
	v.Value = value
	return nil
}

// PreparedPlanCache implements the sessionctx.Context interface.
func (c *Context) PreparedPlanCache() *kvcache.SimpleLRUCache {
	return c.pcache
}

// NewTxn implements the sessionctx.Context interface.
func (c *Context) NewTxn() error {
	if c.Store == nil {
		return errors.New("store is not set")
	}
	if c.txn.Valid() {
		err := c.txn.Commit(c.ctx)
		if err != nil {
			return errors.Trace(err)
		}
	}

	txn, err := c.Store.Begin()
	if err != nil {
		return errors.Trace(err)
	}
	c.txn.Transaction = txn
	return nil
}

// RefreshTxnCtx implements the sessionctx.Context interface.
func (c *Context) RefreshTxnCtx(ctx context.Context) error {
	return errors.Trace(c.NewTxn())
}

// InitTxnWithStartTS implements the sessionctx.Context interface with startTS.
func (c *Context) InitTxnWithStartTS(startTS uint64) error {
	if c.txn.Valid() {
		return nil
	}
	if c.Store != nil {
		membufCap := kv.DefaultTxnMembufCap
		if c.sessionVars.LightningMode {
			membufCap = kv.ImportingTxnMembufCap
		}
		txn, err := c.Store.BeginWithStartTS(startTS)
		if err != nil {
			return errors.Trace(err)
		}
		txn.SetCap(membufCap)
		c.txn.Transaction = txn
	}
	return nil
}

// GetStore gets the store of session.
func (c *Context) GetStore() kv.Storage {
	return c.Store
}

// GetSessionManager implements the sessionctx.Context interface.
func (c *Context) GetSessionManager() util.SessionManager {
	return c.sm
}

// SetSessionManager set the session manager.
func (c *Context) SetSessionManager(sm util.SessionManager) {
	c.sm = sm
}

// Cancel implements the Session interface.
func (c *Context) Cancel() {
	c.cancel()
}

// GoCtx returns standard sessionctx.Context that bind with current transaction.
func (c *Context) GoCtx() context.Context {
	return c.ctx
}

// StoreQueryFeedback stores the query feedback.
func (c *Context) StoreQueryFeedback(_ interface{}) {}

// StmtCommit implements the sessionctx.Context interface.
func (c *Context) StmtCommit() error {
	return nil
}

// StmtRollback implements the sessionctx.Context interface.
func (c *Context) StmtRollback() {
}

// StmtGetMutation implements the sessionctx.Context interface.
func (c *Context) StmtGetMutation(tableID int64) *binlog.TableMutation {
	return nil
}

// StmtAddDirtyTableOP implements the sessionctx.Context interface.
func (c *Context) StmtAddDirtyTableOP(op int, tid int64, handle int64, row []types.Datum) {
}

// NewContext creates a new mocked sessionctx.Context.
func NewContext() *Context {
	ctx, cancel := context.WithCancel(context.Background())
	sctx := &Context{
		values:      make(map[fmt.Stringer]interface{}),
		sessionVars: variable.NewSessionVars(),
		ctx:         ctx,
		cancel:      cancel,
	}
	sctx.sessionVars.MaxChunkSize = 2
	sctx.sessionVars.StmtCtx.TimeZone = time.UTC
	sctx.sessionVars.GlobalVarsAccessor = variable.NewMockGlobalAccessor()
	return sctx
}

// HookKeyForTest is as alias, used by context.WithValue.
// golint forbits using string type as key in context.WithValue.
type HookKeyForTest string
