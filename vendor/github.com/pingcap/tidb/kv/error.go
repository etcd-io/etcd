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

package kv

import (
	"strings"

	"github.com/pingcap/parser/mysql"
	"github.com/pingcap/parser/terror"
)

// KV error codes.
const (
	codeClosed                     terror.ErrCode = 1
	codeNotExist                                  = 2
	codeConditionNotMatch                         = 3
	codeLockConflict                              = 4
	codeLazyConditionPairsNotMatch                = 5
	codeRetryable                                 = 6
	codeCantSetNilValue                           = 7
	codeInvalidTxn                                = 8
	codeNotCommitted                              = 9
	codeNotImplemented                            = 10
	codeTxnTooLarge                               = 11
	codeEntryTooLarge                             = 12

	codeKeyExists = 1062
)

var (
	// ErrClosed is used when close an already closed txn.
	ErrClosed = terror.ClassKV.New(codeClosed, "Error: Transaction already closed")
	// ErrNotExist is used when try to get an entry with an unexist key from KV store.
	ErrNotExist = terror.ClassKV.New(codeNotExist, "Error: key not exist")
	// ErrConditionNotMatch is used when condition is not met.
	ErrConditionNotMatch = terror.ClassKV.New(codeConditionNotMatch, "Error: Condition not match")
	// ErrLockConflict is used when try to lock an already locked key.
	ErrLockConflict = terror.ClassKV.New(codeLockConflict, "Error: Lock conflict")
	// ErrLazyConditionPairsNotMatch is used when value in store differs from expect pairs.
	ErrLazyConditionPairsNotMatch = terror.ClassKV.New(codeLazyConditionPairsNotMatch, "Error: Lazy condition pairs not match")
	// ErrRetryable is used when KV store occurs RPC error or some other
	// errors which SQL layer can safely retry.
	ErrRetryable = terror.ClassKV.New(codeRetryable, "Error: KV error safe to retry")
	// ErrCannotSetNilValue is the error when sets an empty value.
	ErrCannotSetNilValue = terror.ClassKV.New(codeCantSetNilValue, "can not set nil value")
	// ErrInvalidTxn is the error when commits or rollbacks in an invalid transaction.
	ErrInvalidTxn = terror.ClassKV.New(codeInvalidTxn, "invalid transaction")
	// ErrTxnTooLarge is the error when transaction is too large, lock time reached the maximum value.
	ErrTxnTooLarge = terror.ClassKV.New(codeTxnTooLarge, "transaction is too large")
	// ErrEntryTooLarge is the error when a key value entry is too large.
	ErrEntryTooLarge = terror.ClassKV.New(codeEntryTooLarge, "entry is too large")

	// ErrNotCommitted is the error returned by CommitVersion when this
	// transaction is not committed.
	ErrNotCommitted = terror.ClassKV.New(codeNotCommitted, "this transaction has not committed")

	// ErrKeyExists returns when key is already exist.
	ErrKeyExists = terror.ClassKV.New(codeKeyExists, "key already exist")
	// ErrNotImplemented returns when a function is not implemented yet.
	ErrNotImplemented = terror.ClassKV.New(codeNotImplemented, "not implemented")
)

func init() {
	kvMySQLErrCodes := map[terror.ErrCode]uint16{
		codeKeyExists:     mysql.ErrDupEntry,
		codeEntryTooLarge: mysql.ErrTooBigRowsize,
		codeTxnTooLarge:   mysql.ErrTxnTooLarge,
	}
	terror.ErrClassToMySQLCodes[terror.ClassKV] = kvMySQLErrCodes
}

// IsRetryableError checks if the err is a fatal error and the under going operation is worth to retry.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if ErrRetryable.Equal(err) ||
		ErrLockConflict.Equal(err) ||
		ErrConditionNotMatch.Equal(err) ||
		// TiKV exception message will tell you if you should retry or not
		strings.Contains(err.Error(), "try again later") {
		return true
	}

	return false
}

// IsErrNotFound checks if err is a kind of NotFound error.
func IsErrNotFound(err error) bool {
	if ErrNotExist.Equal(err) {
		return true
	}

	return false
}
