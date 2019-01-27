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

package mocktikv

import "fmt"

// ErrLocked is returned when trying to Read/Write on a locked key. Client should
// backoff or cleanup the lock then retry.
type ErrLocked struct {
	Key     MvccKey
	Primary []byte
	StartTS uint64
	TTL     uint64
}

// Error formats the lock to a string.
func (e *ErrLocked) Error() string {
	return fmt.Sprintf("key is locked, key: %q, primary: %q, startTS: %v", e.Key, e.Primary, e.StartTS)
}

// ErrRetryable suggests that client may restart the txn. e.g. write conflict.
type ErrRetryable string

func (e ErrRetryable) Error() string {
	return fmt.Sprintf("retryable: %s", string(e))
}

// ErrAbort means something is wrong and client should abort the txn.
type ErrAbort string

func (e ErrAbort) Error() string {
	return fmt.Sprintf("abort: %s", string(e))
}

// ErrAlreadyCommitted is returned specially when client tries to rollback a
// committed lock.
type ErrAlreadyCommitted uint64

func (e ErrAlreadyCommitted) Error() string {
	return fmt.Sprint("txn already committed")
}
