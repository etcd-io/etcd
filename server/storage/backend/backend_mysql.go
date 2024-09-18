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
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
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
		db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
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
	if err != nil {
		return fmt.Errorf("failed to create kv_store table: %w", err)
	}
	return nil
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
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := m.db.Exec(fmt.Sprintf("CREATE TABLE snapshot_%s AS SELECT * FROM kv_store", id))
	if err != nil {
		m.lg.Error("failed to create snapshot", zap.Error(err))
		return nil
	}
	return &mysqlSnapshot{be: m, id: id}
}

func (m *mysqlBackend) Hash(ignores func([]byte, []byte) bool) (uint32, error) {
	return 0, fmt.Errorf("Hash not implemented for MySQL backend")
}

func (m *mysqlBackend) Size() int64 {
	var size int64
	err := m.db.QueryRow("SELECT SUM(DATA_LENGTH + INDEX_LENGTH) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()").Scan(&size)
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
	return 0 // MySQL doesn't have a concept of read transactions
}

func (m *mysqlBackend) Defrag() error {
	m.lg.Info("Defrag called on MySQL backend (no-op)")
	return nil // MySQL handles fragmentation internally
}

func (m *mysqlBackend) ForceCommit() {
	m.lg.Info("ForceCommit called on MySQL backend (no-op)")
	// MySQL commits automatically, so this is a no-op
}

func (m *mysqlBackend) Close() error {
	close(m.stopc)
	<-m.donec
	if err := m.db.Close(); err != nil {
		return fmt.Errorf("failed to close MySQL connection: %w", err)
	}
	return nil
}

func (m *mysqlBackend) RestoreSnapshot(r io.Reader) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.Exec("DELETE FROM kv_store"); err != nil {
		return fmt.Errorf("failed to clear kv_store: %w", err)
	}

	stmt, err := tx.Prepare("INSERT INTO kv_store (key, value) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

	buf := make([]byte, 8192)
	for {
		n, err := r.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read from snapshot: %w", err)
		}
		key := buf[:n/2]
		value := buf[n/2:n]
		if _, err = stmt.Exec(key, value); err != nil {
			return fmt.Errorf("failed to insert key-value pair: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (m *mysqlBackend) Backup(w io.Writer) error {
	rows, err := m.db.Query("SELECT key, value FROM kv_store")
	if err != nil {
		return fmt.Errorf("failed to query kv_store: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		if _, err := w.Write(key); err != nil {
			return fmt.Errorf("failed to write key: %w", err)
		}
		if _, err := w.Write(value); err != nil {
			return fmt.Errorf("failed to write value: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error during row iteration: %w", err)
	}
	return nil
}

func (m *mysqlBackend) RestoreBackup(r io.Reader) error {
	return m.RestoreSnapshot(r) // The process is the same as restoring from a snapshot
}

func (m *mysqlBackend) SetTxPostLockInsideApplyHook(hook func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.postLockInsideApplyHook = hook
}

type mysqlBatchTx struct {
	be *mysqlBackend
	tx *sql.Tx
}

func (t *mysqlBatchTx) Lock()    { t.be.mu.Lock() }
func (t *mysqlBatchTx) Unlock()  { t.be.mu.Unlock() }

func (t *mysqlBatchTx) UnsafeCreateBucket(bucket Bucket) {
	t.be.lg.Warn("UnsafeCreateBucket called on MySQL backend (no-op)", zap.String("bucket", bucket.String()))
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
		t.be.lg.Error("failed to put key-value pair", zap.Error(err), zap.Binary("key", key))
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
		t.be.lg.Error("failed to delete key", zap.Error(err), zap.Binary("key", key))
	}
}

func (t *mysqlBatchTx) UnsafeDeleteBucket(bucket Bucket) {
	t.be.lg.Warn("UnsafeDeleteBucket called on MySQL backend (no-op)", zap.String("bucket", bucket.String()))
}

func (t *mysqlBatchTx) UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error {
	rows, err := t.be.db.Query("SELECT key, value FROM kv_store")
	if err != nil {
		return fmt.Errorf("failed to query kv_store: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		if err := visitor(k, v); err != nil {
			return fmt.Errorf("visitor function failed: %w", err)
		}
	}
	return rows.Err()
}

func (t *mysqlBatchTx) UnsafeRange(bucket Bucket, key, endKey []byte, limit int64) ([][]byte, [][]byte) {
	var keys, values [][]byte
	query := "SELECT key, value FROM kv_store WHERE key >= ? AND key < ? ORDER BY key LIMIT ?"
	rows, err := t.be.db.Query(query, key, endKey, limit)
	if err != nil {
		t.be.lg.Error("failed to query range", zap.Error(err), zap.Binary("start", key), zap.Binary("end", endKey))
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
	if err := rows.Err(); err != nil {
		t.be.lg.Error("error during row iteration", zap.Error(err))
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
		t.be.lg.Error("failed to query range", zap.Error(err), zap.Binary("start", key), zap.Binary("end", endKey))
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
	if err := rows.Err(); err != nil {
		t.be.lg.Error("error during row iteration", zap.Error(err))
	}
	return keys, values
}

func (t *mysqlReadTx) UnsafeForEach(bucket Bucket, visitor func(k, v []byte) error) error {
	rows, err := t.be.db.Query("SELECT key, value FROM kv_store")
	if err != nil {
		return fmt.Errorf("failed to query kv_store: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var k, v []byte
		if err := rows.Scan(&k, &v); err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}
		if err := visitor(k, v); err != nil {
			return fmt.Errorf("visitor function failed: %w", err)
		}
	}
	return rows.Err()
}

type mysqlSnapshot struct {
	be *mysqlBackend
	id string
}

func (s *mysqlSnapshot) Close() error {
	_, err := s.be.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS snapshot_%s", s.id))
	if err != nil {
		return fmt.Errorf("failed to drop snapshot table: %w", err)
	}
	return nil
}

func (s *mysqlSnapshot) Size() int64 {
	var size int64
	err := s.be.db.QueryRow(fmt.Sprintf("SELECT SUM(LENGTH(key) + LENGTH(value)) FROM snapshot_%s", s.id)).Scan(&size)
	if err != nil {
		s.be.lg.Error("failed to get snapshot size", zap.Error(err))
		return 0
	}
	return size
}

func (s *mysqlSnapshot) WriteTo(w io.Writer) (int64, error) {
	rows, err := s.be.db.Query(fmt.Sprintf("SELECT key, value FROM snapshot_%s", s.id))
	if err != nil {
		return 0, fmt.Errorf("failed to query snapshot: %w", err)
	}
	defer rows.Close()

	var total int64
	for rows.Next() {
		var key, value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return total, fmt.Errorf("failed to scan row: %w", err)
		}
		n, err := w.Write(key)
		total += int64(n)
		if err != nil {
			return total, fmt.Errorf("failed to write key: %w", err)
		}
		n, err = w.Write(value)
		total += int64(n)
		if err != nil {
			return total, fmt.Errorf("failed to write value: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return total, fmt.Errorf("error during row iteration: %w", err)
	}
	return total, nil
}
