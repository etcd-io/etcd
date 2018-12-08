/*
 * Copyright 2017 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package badger

import (
	"bytes"
	"math"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/y"
	farm "github.com/dgryski/go-farm"
	"github.com/pkg/errors"
)

type oracle struct {
	// curRead must be at the top for memory alignment. See issue #311.
	curRead   uint64 // Managed by the mutex.
	refCount  int64
	isManaged bool // Does not change value, so no locking required.

	sync.Mutex
	writeLock  sync.Mutex
	nextCommit uint64

	// Either of these is used to determine which versions can be permanently
	// discarded during compaction.
	discardTs uint64      // Used by ManagedDB.
	readMark  y.WaterMark // Used by DB.

	// commits stores a key fingerprint and latest commit counter for it.
	// refCount is used to clear out commits map to avoid a memory blowup.
	commits map[uint64]uint64
}

func (o *oracle) addRef() {
	atomic.AddInt64(&o.refCount, 1)
}

func (o *oracle) decrRef() {
	if count := atomic.AddInt64(&o.refCount, -1); count == 0 {
		// Clear out commits maps to release memory.
		o.Lock()
		// Avoids the race where something new is added to commitsMap
		// after we check refCount and before we take Lock.
		if atomic.LoadInt64(&o.refCount) != 0 {
			o.Unlock()
			return
		}
		if len(o.commits) >= 1000 { // If the map is still small, let it slide.
			o.commits = make(map[uint64]uint64)
		}
		o.Unlock()
	}
}

func (o *oracle) readTs() uint64 {
	if o.isManaged {
		return math.MaxUint64
	}
	return atomic.LoadUint64(&o.curRead)
}

func (o *oracle) commitTs() uint64 {
	o.Lock()
	defer o.Unlock()
	return o.nextCommit
}

// Any deleted or invalid versions at or below ts would be discarded during
// compaction to reclaim disk space in LSM tree and thence value log.
func (o *oracle) setDiscardTs(ts uint64) {
	o.Lock()
	defer o.Unlock()
	o.discardTs = ts
}

func (o *oracle) discardAtOrBelow() uint64 {
	if o.isManaged {
		o.Lock()
		defer o.Unlock()
		return o.discardTs
	}
	return o.readMark.MinReadTs()
}

// hasConflict must be called while having a lock.
func (o *oracle) hasConflict(txn *Txn) bool {
	if len(txn.reads) == 0 {
		return false
	}
	for _, ro := range txn.reads {
		if ts, has := o.commits[ro]; has && ts > txn.readTs {
			return true
		}
	}
	return false
}

func (o *oracle) newCommitTs(txn *Txn) uint64 {
	o.Lock()
	defer o.Unlock()

	if o.hasConflict(txn) {
		return 0
	}

	var ts uint64
	if !o.isManaged {
		// This is the general case, when user doesn't specify the read and commit ts.
		ts = o.nextCommit
		o.nextCommit++

	} else {
		// If commitTs is set, use it instead.
		ts = txn.commitTs
	}

	for _, w := range txn.writes {
		o.commits[w] = ts // Update the commitTs.
	}
	return ts
}

func (o *oracle) doneCommit(cts uint64) {
	if o.isManaged {
		// No need to update anything.
		return
	}

	for {
		curRead := atomic.LoadUint64(&o.curRead)
		if cts <= curRead {
			return
		}
		atomic.CompareAndSwapUint64(&o.curRead, curRead, cts)
	}
}

// Txn represents a Badger transaction.
type Txn struct {
	readTs   uint64
	commitTs uint64

	update bool     // update is used to conditionally keep track of reads.
	reads  []uint64 // contains fingerprints of keys read.
	writes []uint64 // contains fingerprints of keys written.

	pendingWrites map[string]*Entry // cache stores any writes done by txn.

	db        *DB
	callbacks []func()
	discarded bool

	size         int64
	count        int64
	numIterators int32
}

type pendingWritesIterator struct {
	entries  []*Entry
	nextIdx  int
	readTs   uint64
	reversed bool
}

func (pi *pendingWritesIterator) Next() {
	pi.nextIdx++
}

func (pi *pendingWritesIterator) Rewind() {
	pi.nextIdx = 0
}

func (pi *pendingWritesIterator) Seek(key []byte) {
	key = y.ParseKey(key)
	pi.nextIdx = sort.Search(len(pi.entries), func(idx int) bool {
		cmp := bytes.Compare(pi.entries[idx].Key, key)
		if !pi.reversed {
			return cmp >= 0
		}
		return cmp <= 0
	})
}

func (pi *pendingWritesIterator) Key() []byte {
	y.AssertTrue(pi.Valid())
	entry := pi.entries[pi.nextIdx]
	return y.KeyWithTs(entry.Key, pi.readTs)
}

func (pi *pendingWritesIterator) Value() y.ValueStruct {
	y.AssertTrue(pi.Valid())
	entry := pi.entries[pi.nextIdx]
	return y.ValueStruct{
		Value:     entry.Value,
		Meta:      entry.meta,
		UserMeta:  entry.UserMeta,
		ExpiresAt: entry.ExpiresAt,
		Version:   pi.readTs,
	}
}

func (pi *pendingWritesIterator) Valid() bool {
	return pi.nextIdx < len(pi.entries)
}

func (pi *pendingWritesIterator) Close() error {
	return nil
}

func (txn *Txn) newPendingWritesIterator(reversed bool) *pendingWritesIterator {
	if !txn.update || len(txn.pendingWrites) == 0 {
		return nil
	}
	entries := make([]*Entry, 0, len(txn.pendingWrites))
	for _, e := range txn.pendingWrites {
		entries = append(entries, e)
	}
	// Number of pending writes per transaction shouldn't be too big in general.
	sort.Slice(entries, func(i, j int) bool {
		cmp := bytes.Compare(entries[i].Key, entries[j].Key)
		if !reversed {
			return cmp < 0
		}
		return cmp > 0
	})
	return &pendingWritesIterator{
		readTs:   txn.readTs,
		entries:  entries,
		reversed: reversed,
	}
}

func (txn *Txn) checkSize(e *Entry) error {
	count := txn.count + 1
	// Extra bytes for version in key.
	size := txn.size + int64(e.estimateSize(txn.db.opt.ValueThreshold)) + 10
	if count >= txn.db.opt.maxBatchCount || size >= txn.db.opt.maxBatchSize {
		return ErrTxnTooBig
	}
	txn.count, txn.size = count, size
	return nil
}

// Set adds a key-value pair to the database.
//
// It will return ErrReadOnlyTxn if update flag was set to false when creating the
// transaction.
//
// The current transaction keeps a reference to the key and val byte slice
// arguments. Users must not modify key and val until the end of the transaction.
func (txn *Txn) Set(key, val []byte) error {
	e := &Entry{
		Key:   key,
		Value: val,
	}
	return txn.SetEntry(e)
}

// SetWithMeta adds a key-value pair to the database, along with a metadata
// byte.
//
// This byte is stored alongside the key, and can be used as an aid to
// interpret the value or store other contextual bits corresponding to the
// key-value pair.
//
// The current transaction keeps a reference to the key and val byte slice
// arguments. Users must not modify key and val until the end of the transaction.
func (txn *Txn) SetWithMeta(key, val []byte, meta byte) error {
	e := &Entry{Key: key, Value: val, UserMeta: meta}
	return txn.SetEntry(e)
}

// SetWithDiscard acts like SetWithMeta, but adds a marker to discard earlier
// versions of the key.
//
// This method is only useful if you have set a higher limit for
// options.NumVersionsToKeep. The default setting is 1, in which case, this
// function doesn't add any more benefit than just calling the normal
// SetWithMeta (or Set) function. If however, you have a higher setting for
// NumVersionsToKeep (in Dgraph, we set it to infinity), you can use this method
// to indicate that all the older versions can be discarded and removed during
// compactions.
//
// The current transaction keeps a reference to the key and val byte slice
// arguments. Users must not modify key and val until the end of the
// transaction.
func (txn *Txn) SetWithDiscard(key, val []byte, meta byte) error {
	e := &Entry{
		Key:      key,
		Value:    val,
		UserMeta: meta,
		meta:     bitDiscardEarlierVersions,
	}
	return txn.SetEntry(e)
}

// SetWithTTL adds a key-value pair to the database, along with a time-to-live
// (TTL) setting. A key stored with a TTL would automatically expire after the
// time has elapsed , and be eligible for garbage collection.
//
// The current transaction keeps a reference to the key and val byte slice
// arguments. Users must not modify key and val until the end of the
// transaction.
func (txn *Txn) SetWithTTL(key, val []byte, dur time.Duration) error {
	expire := time.Now().Add(dur).Unix()
	e := &Entry{Key: key, Value: val, ExpiresAt: uint64(expire)}
	return txn.SetEntry(e)
}

func (txn *Txn) modify(e *Entry) error {
	if !txn.update {
		return ErrReadOnlyTxn
	} else if txn.discarded {
		return ErrDiscardedTxn
	} else if len(e.Key) == 0 {
		return ErrEmptyKey
	} else if len(e.Key) > maxKeySize {
		return exceedsMaxKeySizeError(e.Key)
	} else if int64(len(e.Value)) > txn.db.opt.ValueLogFileSize {
		return exceedsMaxValueSizeError(e.Value, txn.db.opt.ValueLogFileSize)
	}
	if err := txn.checkSize(e); err != nil {
		return err
	}

	fp := farm.Fingerprint64(e.Key) // Avoid dealing with byte arrays.
	txn.writes = append(txn.writes, fp)
	txn.pendingWrites[string(e.Key)] = e
	return nil
}

// SetEntry takes an Entry struct and adds the key-value pair in the struct,
// along with other metadata to the database.
//
// The current transaction keeps a reference to the entry passed in argument.
// Users must not modify the entry until the end of the transaction.
func (txn *Txn) SetEntry(e *Entry) error {
	return txn.modify(e)
}

// Delete deletes a key.
//
// This is done by adding a delete marker for the key at commit timestamp.  Any
// reads happening before this timestamp would be unaffected. Any reads after
// this commit would see the deletion.
//
// The current transaction keeps a reference to the key byte slice argument.
// Users must not modify the key until the end of the transaction.
func (txn *Txn) Delete(key []byte) error {
	e := &Entry{
		Key:  key,
		meta: bitDelete,
	}
	return txn.modify(e)
}

// Get looks for key and returns corresponding Item.
// If key is not found, ErrKeyNotFound is returned.
func (txn *Txn) Get(key []byte) (item *Item, rerr error) {
	if len(key) == 0 {
		return nil, ErrEmptyKey
	} else if txn.discarded {
		return nil, ErrDiscardedTxn
	}

	item = new(Item)
	if txn.update {
		if e, has := txn.pendingWrites[string(key)]; has && bytes.Equal(key, e.Key) {
			if isDeletedOrExpired(e.meta, e.ExpiresAt) {
				return nil, ErrKeyNotFound
			}
			// Fulfill from cache.
			item.meta = e.meta
			item.val = e.Value
			item.userMeta = e.UserMeta
			item.key = key
			item.status = prefetched
			item.version = txn.readTs
			item.expiresAt = e.ExpiresAt
			// We probably don't need to set db on item here.
			return item, nil
		}
		// Only track reads if this is update txn. No need to track read if txn serviced it
		// internally.
		fp := farm.Fingerprint64(key)
		txn.reads = append(txn.reads, fp)
	}

	seek := y.KeyWithTs(key, txn.readTs)
	vs, err := txn.db.get(seek)
	if err != nil {
		return nil, errors.Wrapf(err, "DB::Get key: %q", key)
	}
	if vs.Value == nil && vs.Meta == 0 {
		return nil, ErrKeyNotFound
	}
	if isDeletedOrExpired(vs.Meta, vs.ExpiresAt) {
		return nil, ErrKeyNotFound
	}

	item.key = key
	item.version = vs.Version
	item.meta = vs.Meta
	item.userMeta = vs.UserMeta
	item.db = txn.db
	item.vptr = vs.Value
	item.txn = txn
	item.expiresAt = vs.ExpiresAt
	return item, nil
}

func (txn *Txn) runCallbacks() {
	for _, cb := range txn.callbacks {
		cb()
	}
	txn.callbacks = txn.callbacks[:0]
}

// Discard discards a created transaction. This method is very important and must be called. Commit
// method calls this internally, however, calling this multiple times doesn't cause any issues. So,
// this can safely be called via a defer right when transaction is created.
//
// NOTE: If any operations are run on a discarded transaction, ErrDiscardedTxn is returned.
func (txn *Txn) Discard() {
	if txn.discarded { // Avoid a re-run.
		return
	}
	if atomic.LoadInt32(&txn.numIterators) > 0 {
		panic("Unclosed iterator at time of Txn.Discard.")
	}
	txn.discarded = true
	txn.db.orc.readMark.Done(txn.readTs)
	txn.runCallbacks()
	if txn.update {
		txn.db.orc.decrRef()
	}
}

// Commit commits the transaction, following these steps:
//
// 1. If there are no writes, return immediately.
//
// 2. Check if read rows were updated since txn started. If so, return ErrConflict.
//
// 3. If no conflict, generate a commit timestamp and update written rows' commit ts.
//
// 4. Batch up all writes, write them to value log and LSM tree.
//
// 5. If callback is provided, Badger will return immediately after checking
// for conflicts. Writes to the database will happen in the background.  If
// there is a conflict, an error will be returned and the callback will not
// run. If there are no conflicts, the callback will be called in the
// background upon successful completion of writes or any error during write.
//
// If error is nil, the transaction is successfully committed. In case of a non-nil error, the LSM
// tree won't be updated, so there's no need for any rollback.
func (txn *Txn) Commit(callback func(error)) error {
	if txn.commitTs == 0 && txn.db.opt.managedTxns {
		return ErrManagedTxn
	}
	if txn.discarded {
		return ErrDiscardedTxn
	}
	defer txn.Discard()
	if len(txn.writes) == 0 {
		return nil // Nothing to do.
	}

	state := txn.db.orc
	state.writeLock.Lock()
	commitTs := state.newCommitTs(txn)
	if commitTs == 0 {
		state.writeLock.Unlock()
		return ErrConflict
	}

	entries := make([]*Entry, 0, len(txn.pendingWrites)+1)
	for _, e := range txn.pendingWrites {
		// Suffix the keys with commit ts, so the key versions are sorted in
		// descending order of commit timestamp.
		e.Key = y.KeyWithTs(e.Key, commitTs)
		e.meta |= bitTxn
		entries = append(entries, e)
	}
	e := &Entry{
		Key:   y.KeyWithTs(txnKey, commitTs),
		Value: []byte(strconv.FormatUint(commitTs, 10)),
		meta:  bitFinTxn,
	}
	entries = append(entries, e)

	req, err := txn.db.sendToWriteCh(entries)
	state.writeLock.Unlock()
	if err != nil {
		return err
	}

	// Need to release all locks or writes can get deadlocked.
	txn.runCallbacks()

	if callback == nil {
		// If batchSet failed, LSM would not have been updated. So, no need to rollback anything.

		// TODO: What if some of the txns successfully make it to value log, but others fail.
		// Nothing gets updated to LSM, until a restart happens.
		defer state.doneCommit(commitTs)
		return req.Wait()
	}
	go func() {
		err := req.Wait()
		// Write is complete. Let's call the callback function now.
		state.doneCommit(commitTs)
		callback(err)
	}()
	return nil
}

// NewTransaction creates a new transaction. Badger supports concurrent execution of transactions,
// providing serializable snapshot isolation, avoiding write skews. Badger achieves this by tracking
// the keys read and at Commit time, ensuring that these read keys weren't concurrently modified by
// another transaction.
//
// For read-only transactions, set update to false. In this mode, we don't track the rows read for
// any changes. Thus, any long running iterations done in this mode wouldn't pay this overhead.
//
// Running transactions concurrently is OK. However, a transaction itself isn't thread safe, and
// should only be run serially. It doesn't matter if a transaction is created by one goroutine and
// passed down to other, as long as the Txn APIs are called serially.
//
// When you create a new transaction, it is absolutely essential to call
// Discard(). This should be done irrespective of what the update param is set
// to. Commit API internally runs Discard, but running it twice wouldn't cause
// any issues.
//
//  txn := db.NewTransaction(false)
//  defer txn.Discard()
//  // Call various APIs.
func (db *DB) NewTransaction(update bool) *Txn {
	if db.opt.ReadOnly && update {
		// DB is read-only, force read-only transaction.
		update = false
	}

	txn := &Txn{
		update: update,
		db:     db,
		readTs: db.orc.readTs(),
		count:  1,                       // One extra entry for BitFin.
		size:   int64(len(txnKey) + 10), // Some buffer for the extra entry.
	}
	db.orc.readMark.Begin(txn.readTs)
	if update {
		txn.pendingWrites = make(map[string]*Entry)
		txn.db.orc.addRef()
	}
	return txn
}

// View executes a function creating and managing a read-only transaction for the user. Error
// returned by the function is relayed by the View method.
func (db *DB) View(fn func(txn *Txn) error) error {
	if db.opt.managedTxns {
		return ErrManagedTxn
	}
	txn := db.NewTransaction(false)
	defer txn.Discard()

	return fn(txn)
}

// Update executes a function, creating and managing a read-write transaction
// for the user. Error returned by the function is relayed by the Update method.
func (db *DB) Update(fn func(txn *Txn) error) error {
	if db.opt.managedTxns {
		return ErrManagedTxn
	}
	txn := db.NewTransaction(true)
	defer txn.Discard()

	if err := fn(txn); err != nil {
		return err
	}

	return txn.Commit(nil)
}
