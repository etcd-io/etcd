package backend

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

var testMySQLDSN string

// Define TestBucket
var TestBucket Bucket = testBucket("test")

type testBucket string

func (b testBucket) ID() BucketID {
	return BucketID(0) // You might want to implement a proper ID system
}

func (b testBucket) Name() []byte {
	return []byte(b)
}

func (b testBucket) String() string {
	return string(b)
}

func (b testBucket) IsSafeRangeBucket() bool {
	// Implement this method based on your requirements
	// For testing purposes, we'll return true
	return true
}

func init() {
	// Set up the test MySQL DSN
	// You might want to make this configurable via environment variables
	cfg := mysql.NewConfig()
	cfg.User = "root"
	cfg.Passwd = "password"
	cfg.DBName = "etcd_test"
	cfg.ParseTime = true
	cfg.Loc = time.UTC

	testMySQLDSN = cfg.FormatDSN()
}

func setupTestMySQL(t *testing.T) *mysqlBackend {
	db, err := sql.Open("mysql", testMySQLDSN)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create test database
	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS etcd_test")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	db.Close()

	// Create backend
	lg, _ := zap.NewDevelopment()
	bcfg := BackendConfig{
		Logger:   lg,
		MySQLDSN: testMySQLDSN,
	}

	be, err := newMySQLBackend(bcfg)
	if err != nil {
		t.Fatalf("Failed to create MySQL backend: %v", err)
	}

	return be
}

func teardownTestMySQL(t *testing.T, be *mysqlBackend) {
	be.Close()

	// Drop test database
	db, err := sql.Open("mysql", testMySQLDSN)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("DROP DATABASE IF EXISTS etcd_test")
	if err != nil {
		t.Fatalf("Failed to drop test database: %v", err)
	}
}

func TestMySQLBackend_BatchTx(t *testing.T) {
	be := setupTestMySQL(t)
	defer teardownTestMySQL(t, be)

	tx := be.BatchTx()

	// Test Put and Get
	bucket := TestBucket
	key := []byte("testkey")
	value := []byte("testvalue")

	tx.Lock()
	tx.UnsafePut(bucket, key, value)
	tx.Unlock()

	tx.Commit()

	rtx := be.ReadTx()
	rtx.RLock()
	gotValues, _ := rtx.UnsafeRange(bucket, key, nil, 0)
	rtx.RUnlock()

	if len(gotValues) == 0 || string(gotValues[0]) != string(value) {
		t.Errorf("Got %s, want %s", string(gotValues[0]), string(value))
	}

	// Test Delete
	tx.Lock()
	tx.UnsafeDelete(bucket, key)
	tx.Unlock()

	tx.Commit()

	rtx.RLock()
	gotValues, _ = rtx.UnsafeRange(bucket, key, nil, 0)
	rtx.RUnlock()

	if len(gotValues) != 0 {
		t.Errorf("Got %s, want nil", string(gotValues[0]))
	}
}

func TestMySQLBackend_ReadTx(t *testing.T) {
	be := setupTestMySQL(t)
	defer teardownTestMySQL(t, be)

	tx := be.BatchTx()

	// Insert test data
	bucket := TestBucket
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	tx.Lock()
	for k, v := range testData {
		tx.UnsafePut(bucket, []byte(k), []byte(v))
	}
	tx.Unlock()

	tx.Commit()

	// Test Range
	rtx := be.ReadTx()
	rtx.RLock()
	keys, values := rtx.UnsafeRange(bucket, []byte("key"), []byte("key4"), 0)
	rtx.RUnlock()

	if len(keys) != len(testData) || len(values) != len(testData) {
		t.Errorf("Got %d keys and %d values, want %d each", len(keys), len(values), len(testData))
	}

	for i, key := range keys {
		value := values[i]
		if testData[string(key)] != string(value) {
			t.Errorf("For key %s, got value %s, want %s", string(key), string(value), testData[string(key)])
		}
	}
}

func TestMain(m *testing.M) {
	// Set up any global test environment here if needed

	// Run the tests
	code := m.Run()

	// Tear down any global test environment here if needed

	os.Exit(code)
}

