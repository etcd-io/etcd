package backend

import (
	"database/sql"
	"fmt"
	"io"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

type mysqlBackend struct {
	db                      *sql.DB
	lg                      *zap.Logger
	mu                      sync.RWMutex
	stopc                   chan struct{}
	donec                   chan struct{}
	hooks                   Hooks
	postLockInsideApplyHook func()
}

func newMySQLBackend(bcfg BackendConfig) (*mysqlBackend, error) {
	db, err := sql.Open("mysql", bcfg.MySQLDSN)
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	be := &mysqlBackend{
		db:    db,
		lg:    bcfg.Logger,
		stopc: make(chan struct{}),
		donec: make(chan struct{}),
		hooks: bcfg.Hooks,
	}

	// Initialize tables
	if err := be.initTables(); err != nil {
		return nil, err
	}

	return be, nil
}

func (m *mysqlBackend) initTables() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS kv_store (
			key VARBINARY(512) PRIMARY KEY,
			value LONGBLOB,
			create_revision BIGINT,
			mod_revision BIGINT,
			version BIGINT
		)
	`)
	return err
}

func (m *mysqlBackend) BatchTx() BatchTx {
	return &mysqlBatchTx{be: m}
}

func (m *mysqlBackend) ReadTx() ReadTx {
	return &mysqlReadTx{be: m}
}

func (m *mysqlBackend) ConcurrentReadTx() ReadTx {
	return &mysqlReadTx{be: m}
}

func (m *mysqlBackend) Snapshot() Snapshot {
	return &mysqlSnapshot{be: m}
}

func (m *mysqlBackend) Hash(ignores func([]byte, []byte) bool) (uint32, error) {
	// Implement hash calculation for MySQL
	// This is a placeholder implementation; you should replace it with your actual logic.
	return 0, fmt.Errorf("Hash not implemented for MySQL backend")
}

func (m *mysqlBackend) Size() int64 {
	var size int64
	row := m.db.QueryRow("SELECT SUM(DATA_LENGTH + INDEX_LENGTH) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()")
	err := row.Scan(&size)
	if err != nil {
		m.lg.Error("failed to get database size", zap.Error(err))
		return 0
	}
	return size
}

func (m *mysqlBackend) SizeInUse() int64 {
	return m.Size() // For MySQL, Size and SizeInUse are the same
}

func (m *mysqlBackend) OpenReadTxN() int64 {
	// MySQL doesn't have a concept of read transactions, so return 0
	return 0
}

func (m *mysqlBackend) Defrag() error {
	// MySQL handles fragmentation internally, so this is a no-op
	return nil
}

func (m *mysqlBackend) ForceCommit() {
	// MySQL commits automatically, so this is a no-op
}

func (m *mysqlBackend) Close() error {
	close(m.stopc)
	<-m.donec
	return m.db.Close()
}

func (m *mysqlBackend) SetTxPostLockInsideApplyHook(hook func()) {
	m.postLockInsideApplyHook = hook
}

// mysqlBatchTx implements BatchTx interface
type mysqlBatchTx struct {
	be *mysqlBackend
	tx *sql.Tx
}

func (t *mysqlBatchTx) Lock() {
	t.be.mu.Lock()
}

func (t *mysqlBatchTx) Unlock() {
	t.be.mu.Unlock()
}

func (t *mysqlBatchTx) UnsafeCreateBucket(bucket Bucket) {
	// MySQL doesn't use buckets, so this is a no-op
	t.be.lg.Warn("UnsafeCreateBucket called on MySQL backend", zap.String("bucket", "n/a"))
}

func (t *mysqlBatchTx) UnsafePut(bucket Bucket, key []byte, value []byte) {
	if t.tx == nil {
		var err error
		t.tx, err = t.be.db.Begin()
		if err != nil {
			t.be.lg.Error("failed to begin transaction", zap.Error(err))
			return
		}
	}
	_, err := t.tx.Exec("INSERT INTO kv_store (key, value) VALUES (?, ?) ON DUPLICATE KEY UPDATE value = ?", key, value, value)
	if err != nil {
		t.be.lg.Error("failed to put key-value pair", zap.Error(err))
	}
}

func (t *mysqlBatchTx) UnsafeSeqPut(bucket Bucket, key []byte, value []byte) {
	t.UnsafePut(bucket, key, value)
}

func (t *mysqlBatchTx) UnsafeDelete(bucket Bucket, key []byte) {
	if t.tx == nil {
		var err error
		t.tx, err = t.be.db.Begin()
		if err != nil {
			t.be.lg.Error("failed to begin transaction", zap.Error(err))
			return
		}
	}
	_, err := t.tx.Exec("DELETE FROM kv_store WHERE key = ?", key)
	if err != nil {
		t.be.lg.Error("failed to delete key", zap.Error(err))
	}
}

func (t *mysqlBatchTx) UnsafeDeleteBucket(bucket Bucket) {
	t.be.lg.Warn("UnsafeDeleteBucket called on MySQL backend", zap.String("bucket", "n/a"))
	// No-op for MySQL as it doesn't use buckets
}

func (t *mysqlBatchTx) UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error {
	rows, err := t.be.db.Query("SELECT key, value FROM kv_store")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		if err := visitor(k, v); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (t *mysqlBatchTx) UnsafeRange(bucket Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	var keys, values [][]byte
	query := "SELECT key, value FROM kv_store WHERE key >= ? AND key < ? ORDER BY key LIMIT ?"
	rows, err := t.be.db.Query(query, key, endKey, limit)
	if err != nil {
		t.be.lg.Error("failed to query range", zap.Error(err))
		return nil, nil
	}
	defer rows.Close()
	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			t.be.lg.Error("failed to scan row", zap.Error(err))
			continue
		}
		keys = append(keys, k)
		values = append(values, v)
	}
	return keys, values
}

func (t *mysqlBatchTx) Commit() {
	if t.tx != nil {
		err := t.tx.Commit()
		if err != nil {
			t.be.lg.Error("failed to commit transaction", zap.Error(err))
		}
		t.tx = nil
	}
}

func (t *mysqlBatchTx) CommitAndStop() {
	t.Commit()
	// Additional cleanup if needed
}

func (t *mysqlBatchTx) LockInsideApply() {
	t.be.mu.Lock()
	if t.be.postLockInsideApplyHook != nil {
		t.be.postLockInsideApplyHook()
	}
}

func (t *mysqlBatchTx) LockOutsideApply() {
	t.be.mu.Lock()
}

// mysqlReadTx implements ReadTx interface
type mysqlReadTx struct {
	be *mysqlBackend
}

func (t *mysqlReadTx) Lock()    {}
func (t *mysqlReadTx) Unlock()  {}
func (t *mysqlReadTx) Reset()   {}
func (t *mysqlReadTx) RLock()   {}
func (t *mysqlReadTx) RUnlock() {}

func (t *mysqlReadTx) UnsafeRange(bucket Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	var keys, values [][]byte
	query := "SELECT key, value FROM kv_store WHERE key >= ? AND key < ? ORDER BY key LIMIT ?"
	rows, err := t.be.db.Query(query, key, endKey, limit)
	if err != nil {
		t.be.lg.Error("failed to query range", zap.Error(err))
		return nil, nil
	}
	defer rows.Close()
	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			t.be.lg.Error("failed to scan row", zap.Error(err))
			continue
		}
		keys = append(keys, k)
		values = append(values, v)
	}
	return keys, values
}

func (t *mysqlReadTx) UnsafeGet(bucket Bucket, key []byte) (value []byte, err error) {
	err = t.be.db.QueryRow("SELECT value FROM kv_store WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return
}

func (t *mysqlReadTx) UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error {
	rows, err := t.be.db.Query("SELECT key, value FROM kv_store")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		if err := visitor(k, v); err != nil {
			return err
		}
	}
	return rows.Err()
}

// mysqlSnapshot implements Snapshot interface
type mysqlSnapshot struct {
	be *mysqlBackend
}

func (s *mysqlSnapshot) Close() error {
	// MySQL doesn't require explicit snapshot closing
	return nil
}

func (s *mysqlSnapshot) Size() int64 {
	return s.be.Size()
}

func (s *mysqlSnapshot) WriteTo(w io.Writer) (int64, error) {
	// Implement snapshot writing logic
	return 0, fmt.Errorf("WriteTo not implemented for MySQL snapshot")
}
