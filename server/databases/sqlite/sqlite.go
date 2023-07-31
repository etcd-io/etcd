/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlite

import (
	// sqlite DB driver

	"bufio"
	"database/sql"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"os"
	"strings"

	_ "github.com/glebarez/go-sqlite"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/server/v3/interfaces"
)

const (
	hasBucketQuery               = `SELECT name FROM sqlite_master WHERE type='table' AND name=?;`
	queryTableNames              = `SELECT name FROM sqlite_schema WHERE type='table' ORDER BY name;`
	dropBucketQuery              = `DROP TABLE IF EXISTS ?;`
	createBucketQuery            = "CREATE TABLE IF NOT EXISTS %s (key STRING PRIMARY KEY, value BLOB);"
	genericUnsafeRangeQuery      = "select key, value from %s WHERE key >= ? AND key <= ? ORDER BY key limit ?;"
	genericUnsafeRangeQueryNoEnd = "select key, value from %s WHERE key >= ? ORDER BY key limit ?;"
	genericGet                   = "SELECT value from %s WHERE key=?;"
	genericUpsert                = "INSERT INTO %s (key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value;"
	genericDelete                = "DELETE from %s where key = ?;"
	genericForEach               = "select key, value from %s;"

	sizeQuery     = `SELECT page_count * page_size as size FROM pragma_page_count(), pragma_page_size();`
	defragCommand = `VACUUM;`
	UpsertKV      = `INSERT INTO KVs (key, value)
  VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value;`

	dbName = "db"
)

type SqliteDB struct {
	DB           *sql.DB
	Dir          string
	dbName       string
	FreeListType string // no-opts
}

type BackendBucket interface {
	Name() []byte
}

func NewBlankSqliteDB(dir string) (*SqliteDB, error) {
	parts := strings.Split(dir, "/")
	subdir := strings.Join(parts[:len(parts)-1], "/")
	name := parts[len(parts)-1]
	if err := os.MkdirAll(subdir, 0755); err != nil {
		fmt.Printf("couldn't make directory: %s", dir)
		return nil, err
	}
	db, err := sql.Open("sqlite", subdir+"/"+dbName)

	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(50)
	db.SetMaxOpenConns(50)
	// ensure that DB is functional
	if err = db.Ping(); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	sdb := newDB(db, subdir, name)
	return sdb, nil
}

func NewSqliteDB[B BackendBucket](dir string, buckets ...B) (*SqliteDB, error) {
	parts := strings.Split(dir, "/")
	subdir := strings.Join(parts[:len(parts)-1], "/")
	name := parts[len(parts)-1]
	db, err := sql.Open("sqlite", dir)

	if err != nil {
		return nil, err
	}
	db.SetConnMaxLifetime(0)
	db.SetMaxIdleConns(50)
	db.SetMaxOpenConns(50)
	// ensure that DB is functional
	if err = db.Ping(); err != nil {
		return nil, err
	}
	for _, b := range buckets {
		tn := resolveTableName(string(b.Name()))
		createTableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (key STRING PRIMARY KEY, value BLOB );", tn)
		_, err = db.Exec(createTableQuery)
	}

	if err != nil {
		return nil, err
	}
	sdb := newDB(db, subdir, name)
	return sdb, nil
}

func newDB(db *sql.DB, dir string, dbName string) *SqliteDB {
	return &SqliteDB{
		DB:           db,
		Dir:          dir,
		dbName:       dbName,
		FreeListType: string(bolt.FreelistMapType), // dummy value
	}
}

func (s *SqliteDB) Path() string {
	return s.Dir
}

func (s *SqliteDB) GoString() string {
	return s.Dir + "/" + dbName
}

func (s *SqliteDB) Buckets() []string {
	rows, err := s.DB.Query(queryTableNames)
	if err != nil {
		return nil
	}
	defer rows.Close()
	names := make([]string, 0)

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			// Check for a scan error.
			// Query rows will be closed with defer.
			log.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func (s *SqliteDB) HasBucket(name string) bool {
	tableName := resolveTableName(name)
	rows, err := s.DB.Query(hasBucketQuery, tableName)
	if err != nil {
		return false
	}
	defer rows.Close()
	names := make([]string, 0)

	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			// Check for a scan error.
			// Query rows will be closed with defer.
			log.Fatal(err)
		}
		names = append(names, n)
	}
	if len(names) < 1 {
		return false
	} else if len(names) > 1 {
		panic("too many keys of the same bucket")
	} else {
		return true
	}
}

func (s *SqliteDB) DeleteBucket(name []byte) error {
	tableName := resolveTableName(string(name))
	_, err := s.DB.Exec(dropBucketQuery, tableName)
	if err != nil {
		return err
	}
	return nil
}

func (s *SqliteDB) CreateBucket(s2 string) {
	tableName := resolveTableName(s2)
	query := fmt.Sprintf(createBucketQuery, tableName)
	if _, err := s.DB.Exec(query); err != nil {
		panic(err)
	}
}

func (s *SqliteDB) GetFromBucket(bucket string, key string) []byte {
	tableName := resolveTableName(bucket)
	query := fmt.Sprintf(genericGet, tableName)
	r, err := s.DB.Query(query, key)
	if err != nil {
		return nil
	}
	defer r.Close()
	var val []byte
	for r.Next() {
		if err := r.Scan(&val); err != nil {
			log.Fatal(err)
		}
		return val
	}
	return nil
}

func resolveTableName(bucket string) string {
	tableName := bucket
	if tableName == "key" {
		tableName = "KVs"
	}
	return tableName
}

func (s *SqliteDB) String() string {
	//TODO implement me
	panic("implement me")
}

func (s *SqliteDB) Close() error {
	return s.DB.Close()
}

func (s *SqliteDB) Begin(writable bool) (interfaces.Tx, error) {
	tx, err := s.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &SqliteTx{tx: tx, writable: writable, db: s.DB, dir: s.Dir, dbName: s.dbName}, nil
}

func (s *SqliteDB) Size() (size int64) {
	if rows, err := s.DB.Query(sizeQuery); err != nil {
		return 0
	} else {
		var val int64
		defer rows.Close()
		rows.Next()
		rows.Scan(&val)
		return val
	}
}

func (s *SqliteDB) Sync() error {
	// no-opt.
	return nil
}

func (s *SqliteDB) Stats() interface{} {
	return s.DB.Stats()
}

func (s *SqliteDB) Info() interface{} {
	return s.DB.Stats()
}

func (s *SqliteDB) SetFreelistType(freelistType string) {
	s.FreeListType = freelistType
}

func (s *SqliteDB) FreelistType() string {
	return s.FreeListType
}

func (s *SqliteDB) DBType() string {
	return "sqlite"
}

func (s *SqliteDB) HashBuckets(ignores func(bucketName []byte, keyName []byte) bool) (uint32, error) {
	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	// todo(logicalhan) fixme
	return h.Sum32(), nil
}

func (s *SqliteDB) Defrag(logger *zap.Logger, dbopts interface{}, limit int) error {
	_, err := s.DB.Exec(defragCommand)
	return err
}

type SqliteTx struct {
	tx       *sql.Tx
	db       *sql.DB
	dir      string
	dbName   string
	writable bool
	size     int64
}

func (s *SqliteTx) DB() interfaces.DB {
	return newDB(s.db, s.dir, s.dbName)
}

func (s *SqliteTx) Size() int64 {
	if s.size == 0 {
		return s.DB().Size()
	}
	return s.size
}

func (s *SqliteTx) Writable() bool {
	return s.writable
}

func (s *SqliteTx) Stats() interface{} {
	panic("implement me")
}

func (s *SqliteTx) Bucket(name []byte) interfaces.Bucket {
	tableName := resolveTableName(string(name))
	return &SqliteBucket{
		name:     tableName,
		db:       s.db,
		dbName:   s.dbName,
		TX:       s.tx,
		dir:      s.dir,
		writable: s.writable,
	}
}

func (s *SqliteTx) CreateBucket(name []byte) (interfaces.Bucket, error) {
	tableName := resolveTableName(string(name))
	query := fmt.Sprintf(createBucketQuery, tableName)
	_, err := s.tx.Exec(query)
	if err != nil {
		return nil, err
	}
	return &SqliteBucket{
		name:     tableName,
		db:       s.db,
		TX:       s.tx,
		dbName:   s.dbName,
		dir:      s.dir,
		writable: s.writable,
	}, nil
}

func (s *SqliteTx) DeleteBucket(name []byte) error {
	_, err := s.tx.Exec(dropBucketQuery, string(name))
	return err
}

func (s *SqliteTx) ForEach(i interface{}) error {
	//TODO implement me
	panic("implement me")
}

func (s *SqliteTx) Observe(rebalanceHist, spillHist, writeHist prometheus.Histogram) {
	// no-opt
}

func (s *SqliteTx) WriteTo(w io.Writer) (n int64, err error) {
	tmpdir := os.TempDir()
	os.MkdirAll(tmpdir, 0755)
	backup := tmpdir + "etcd.sqlite"
	os.Remove(backup)
	if _, err := s.db.Exec(`VACUUM main INTO ?;`, backup); err != nil {
		return 0, err
	}
	stat, err := os.Stat(backup)
	if err != nil {
		return 0, err
	}
	size := stat.Size()
	s.size = size
	f, err := os.Open(backup)
	defer f.Close()
	if err != nil {
		return 0, err
	}

	r := bufio.NewReader(f)
	return r.WriteTo(w)
}

func (s *SqliteTx) CopyDatabase(lg *zap.Logger, dst string) (err error) {
	//TODO implement me
	panic("implement me")
}

func (s *SqliteTx) Commit() error {
	return s.tx.Commit()
}

func (s *SqliteTx) Rollback() error {
	return s.tx.Rollback()
}

type SqliteBucket struct {
	TX       *sql.Tx
	name     string
	dbName   string
	db       *sql.DB
	dir      string
	writable bool
}

func (s *SqliteBucket) Tx() interfaces.Tx {
	return &SqliteTx{
		tx:       s.TX,
		db:       s.db,
		dbName:   s.dbName,
		dir:      s.dir,
		writable: s.writable,
	}
}

func (s *SqliteBucket) Writable() bool {
	return s.writable
}

func (s *SqliteBucket) Get(key []byte) []byte {
	query := fmt.Sprintf(genericGet, s.name)
	r, err := s.TX.Query(query, string(key))
	defer r.Close()
	if err != nil {
		return nil
	}
	val := []byte{}

	for r.Next() {
		var v []byte
		if err := r.Scan(&v); err != nil {
			// Check for a scan error.
			// Query rows will be closed with defer.
			log.Fatal(err)
		}
		val = append(val, v...)
	}
	return val
}

func (s *SqliteBucket) Put(key []byte, value []byte) error {
	query := fmt.Sprintf(genericUpsert, s.name)
	_, err := s.TX.Exec(query, string(key), value)
	return err
}

func (s *SqliteBucket) UnsafeRange(key, endKey []byte, limit int64) (keys [][]byte, vs [][]byte) {
	if endKey == nil || limit == 0 || limit == 1 {
		query := fmt.Sprintf(genericGet, s.name)
		r, err := s.TX.Query(query, string(key))
		defer r.Close()
		if err != nil {
			return
		}
		for r.Next() {
			var val []byte
			r.Scan(&val)
			keys = append(keys, key)
			vs = append(vs, val)
			return
		}
	}
	var query string
	var r *sql.Rows
	var err error
	if endKey == nil {
		query = fmt.Sprintf(genericUnsafeRangeQueryNoEnd, s.name)
		r, err = s.TX.Query(query, string(key), limit)
	} else {
		query := fmt.Sprintf(genericUnsafeRangeQuery, s.name)
		r, err = s.TX.Query(query, string(key), string(endKey), limit)
	}

	if err != nil {
		return nil, nil
	}
	defer r.Close()
	names := make([][]byte, 0)
	values := make([][]byte, 0)
	for r.Next() {
		var key string
		var v []byte
		if err := r.Scan(&key, &v); err != nil {
			// Check for a scan error.
			// Query rows will be closed with defer.
			log.Fatal(err)
		}
		names = append(names, []byte(key))
		values = append(values, v)
	}
	return names, values
}

func (s *SqliteBucket) Delete(key []byte) error {
	query := fmt.Sprintf(genericDelete, s.name)
	_, err := s.TX.Exec(query, string(key))
	return err
}

func (s *SqliteBucket) ForEach(f func(k []byte, v []byte) error) error {
	query := fmt.Sprintf(genericForEach, s.name)
	r, err := s.TX.Query(query)
	defer r.Close()
	if err != nil {
		return err
	}
	for r.Next() {
		var key string
		var v []byte
		if err := r.Scan(&key, &v); err != nil {
			return err
		}
		if err := f([]byte(key), v); err != nil {
			return err
		}

	}
	return err
}

func (s *SqliteBucket) Stats() interface{} {
	return nil
}

func (s *SqliteBucket) SetFillPercent(f float64) {
	return
}
