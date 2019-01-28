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
	"math"
	"math/rand"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/parser/terror"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// ContextKey is the type of context's key
type ContextKey string

// RunInNewTxn will run the f in a new transaction environment.
func RunInNewTxn(store Storage, retryable bool, f func(txn Transaction) error) error {
	var (
		err           error
		originalTxnTS uint64
		txn           Transaction
	)
	for i := uint(0); i < maxRetryCnt; i++ {
		txn, err = store.Begin()
		if err != nil {
			log.Errorf("[kv] RunInNewTxn error - %v", err)
			return errors.Trace(err)
		}

		// originalTxnTS is used to trace the original transaction when the function is retryable.
		if i == 0 {
			originalTxnTS = txn.StartTS()
		}

		err = f(txn)
		if err != nil {
			err1 := txn.Rollback()
			terror.Log(errors.Trace(err1))
			if retryable && IsRetryableError(err) {
				log.Warnf("[kv] Retry txn %v original txn %v err %v", txn, originalTxnTS, err)
				continue
			}
			return errors.Trace(err)
		}

		err = txn.Commit(context.Background())
		if err == nil {
			break
		}
		if retryable && IsRetryableError(err) {
			log.Warnf("[kv] Retry txn %v original txn %v err %v", txn, originalTxnTS, err)
			BackOff(i)
			continue
		}
		return errors.Trace(err)
	}
	return errors.Trace(err)
}

var (
	// maxRetryCnt represents maximum retry times in RunInNewTxn.
	maxRetryCnt uint = 100
	// retryBackOffBase is the initial duration, in microsecond, a failed transaction stays dormancy before it retries
	retryBackOffBase = 1
	// retryBackOffCap is the max amount of duration, in microsecond, a failed transaction stays dormancy before it retries
	retryBackOffCap = 100
)

// BackOff Implements exponential backoff with full jitter.
// Returns real back off time in microsecond.
// See http://www.awsarchitectureblog.com/2015/03/backoff.html.
func BackOff(attempts uint) int {
	upper := int(math.Min(float64(retryBackOffCap), float64(retryBackOffBase)*math.Pow(2.0, float64(attempts))))
	sleep := time.Duration(rand.Intn(upper)) * time.Millisecond
	time.Sleep(sleep)
	return int(sleep)
}

// BatchGetValues gets values in batch.
// The values from buffer in transaction and the values from the storage node are merged together.
func BatchGetValues(txn Transaction, keys []Key) (map[string][]byte, error) {
	if txn.IsReadOnly() {
		return txn.GetSnapshot().BatchGet(keys)
	}
	bufferValues := make([][]byte, len(keys))
	shrinkKeys := make([]Key, 0, len(keys))
	for i, key := range keys {
		val, err := txn.GetMemBuffer().Get(key)
		if IsErrNotFound(err) {
			shrinkKeys = append(shrinkKeys, key)
			continue
		}
		if err != nil {
			return nil, errors.Trace(err)
		}
		if len(val) != 0 {
			bufferValues[i] = val
		}
	}
	storageValues, err := txn.GetSnapshot().BatchGet(shrinkKeys)
	if err != nil {
		return nil, errors.Trace(err)
	}
	for i, key := range keys {
		if bufferValues[i] == nil {
			continue
		}
		storageValues[string(key)] = bufferValues[i]
	}
	return storageValues, nil
}
