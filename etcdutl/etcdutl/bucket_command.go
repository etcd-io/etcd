// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcdutl

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/authpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/lease/leasepb"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/schema"
)

var (
	// TODO: add this configure to top level etcdutl command
	flockTimeout        time.Duration
	iterateBucketLimit  uint64
	iterateBucketDecode bool
)

func NewListBucketCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-bucket [data dir or db file path]",
		Short: "bucket lists all buckets.",
		Args:  cobra.ExactArgs(1),
		Run:   listBucketCommandFunc,
	}

	// TODO: add this flag to top level etctutl command
	cmd.PersistentFlags().DurationVar(&flockTimeout, "timeout", 10*time.Second, "time to wait to obtain a file lock on db file, 0 to block indefinitely")

	return cmd
}

func NewIterateBucketCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iterate-bucket [data dir or db file path] [bucket name]",
		Short: "iterate-bucket lists key-value pairs in reverse order.",
		Args:  cobra.ExactArgs(2),
		Run:   iterateBucketCommandFunc,
	}

	// TODO: add this flag to top level etctutl command
	cmd.PersistentFlags().DurationVar(&flockTimeout, "timeout", 10*time.Second, "time to wait to obtain a file lock on db file, 0 to block indefinitely")
	cmd.PersistentFlags().Uint64Var(&iterateBucketLimit, "limit", 0, "max number of key-value pairs to iterate (0 to iterate all)")
	cmd.PersistentFlags().BoolVar(&iterateBucketDecode, "decode", false, "true to decode Protocol Buffer encoded data")

	return cmd
}

func NewHashCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hash [data dir or db file path]",
		Short: "hash computes the hash of db file.",
		Args:  cobra.ExactArgs(1),
		Run:   getHashCommandFunc,
	}

	// TODO: add this flag to top level etctutl command
	cmd.PersistentFlags().DurationVar(&flockTimeout, "timeout", 10*time.Second, "time to wait to obtain a file lock on db file, 0 to block indefinitely")

	return cmd
}

func listBucketCommandFunc(_ *cobra.Command, args []string) {
	lg := GetLogger()
	dp := args[0]
	if !strings.HasSuffix(dp, "db") {
		dp = filepath.Join(datadir.ToSnapDir(dp), "db")
	}

	if !fileutil.Exist(dp) {
		lg.Fatal("db file not exist", zap.String("path", dp))
	}

	bts, err := getBuckets(dp)
	if err != nil {
		lg.Fatal("Failed to get buckets", zap.Error(err))
	}
	for _, b := range bts {
		fmt.Println(b)
	}
}

func getBuckets(dbPath string) (buckets []string, err error) {
	db, derr := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: flockTimeout})
	if derr != nil {
		return nil, fmt.Errorf("failed to open bolt DB %w", derr)
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(b []byte, _ *bolt.Bucket) error {
			buckets = append(buckets, string(b))
			return nil
		})
	})
	return buckets, err
}

func iterateBucketCommandFunc(_ *cobra.Command, args []string) {
	lg := GetLogger()
	dp := args[0]
	if !strings.HasSuffix(dp, "db") {
		dp = filepath.Join(datadir.ToSnapDir(dp), "db")
	}
	if !fileutil.Exist(dp) {
		lg.Fatal("db file not exist", zap.String("path", dp))
	}
	bucket := args[1]
	err := iterateBucket(dp, bucket, iterateBucketLimit, iterateBucketDecode)
	if err != nil {
		lg.Fatal("Failed to iterate bucket", zap.Error(err))
	}
}

type decoder func(k, v []byte)

// key is the bucket name, and value is the function to decode K/V in the bucket.
var decoders = map[string]decoder{
	"key":       keyDecoder,
	"lease":     leaseDecoder,
	"auth":      authDecoder,
	"authRoles": authRolesDecoder,
	"authUsers": authUsersDecoder,
	"meta":      metaDecoder,
}

func defaultDecoder(k, v []byte) {
	fmt.Printf("key=%q, value=%q\n", k, v)
}

func keyDecoder(k, v []byte) {
	rev := mvcc.BytesToBucketKey(k)
	var kv mvccpb.KeyValue
	if err := kv.Unmarshal(v); err != nil {
		panic(err)
	}
	fmt.Printf("rev=%+v, value=[key %q | val %q | created %d | mod %d | ver %d]\n", rev, string(kv.Key), string(kv.Value), kv.CreateRevision, kv.ModRevision, kv.Version)
}

func bytesToLeaseID(bytes []byte) int64 {
	if len(bytes) != 8 {
		panic(fmt.Errorf("lease ID must be 8-byte"))
	}
	return int64(binary.BigEndian.Uint64(bytes))
}

func leaseDecoder(k, v []byte) {
	leaseID := bytesToLeaseID(k)
	var lpb leasepb.Lease
	if err := lpb.Unmarshal(v); err != nil {
		panic(err)
	}
	fmt.Printf("lease ID=%016x, TTL=%ds, remaining TTL=%ds\n", leaseID, lpb.TTL, lpb.RemainingTTL)
}

func authDecoder(k, v []byte) {
	if string(k) == "authRevision" {
		rev := binary.BigEndian.Uint64(v)
		fmt.Printf("key=%q, value=%v\n", k, rev)
	} else {
		fmt.Printf("key=%q, value=%v\n", k, v)
	}
}

func authRolesDecoder(_, v []byte) {
	role := &authpb.Role{}
	err := role.Unmarshal(v)
	if err != nil {
		panic(err)
	}
	fmt.Printf("role=%q, keyPermission=%v\n", string(role.Name), role.KeyPermission)
}

func authUsersDecoder(_, v []byte) {
	user := &authpb.User{}
	err := user.Unmarshal(v)
	if err != nil {
		panic(err)
	}
	fmt.Printf("user=%q, roles=%q, option=%v\n", user.Name, user.Roles, user.Options)
}

func metaDecoder(k, v []byte) {
	if string(k) == string(schema.MetaConsistentIndexKeyName) || string(k) == string(schema.MetaTermKeyName) {
		fmt.Printf("key=%q, value=%v\n", k, binary.BigEndian.Uint64(v))
	} else if string(k) == string(schema.ScheduledCompactKeyName) || string(k) == string(schema.FinishedCompactKeyName) {
		rev := mvcc.BytesToRev(v)
		fmt.Printf("key=%q, value=%v\n", k, rev)
	} else {
		defaultDecoder(k, v)
	}
}

func iterateBucket(dbPath, bucket string, limit uint64, decode bool) (err error) {
	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: flockTimeout})
	if err != nil {
		return fmt.Errorf("failed to open bolt DB %w", err)
	}
	defer db.Close()

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("got nil bucket for %s", bucket)
		}

		c := b.Cursor()

		// iterate in reverse order (use First() and Next() for ascending order)
		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			// TODO: remove sensitive information
			// (https://github.com/etcd-io/etcd/issues/7620)
			if dec, ok := decoders[bucket]; decode && ok {
				dec(k, v)
			} else {
				defaultDecoder(k, v)
			}

			limit--
			if limit == 0 {
				break
			}
		}

		return nil
	})
	return err
}

func getHashCommandFunc(_ *cobra.Command, args []string) {
	lg := GetLogger()
	dp := args[0]
	if !strings.HasSuffix(dp, "db") {
		dp = filepath.Join(datadir.ToSnapDir(dp), "db")
	}
	if !fileutil.Exist(dp) {
		lg.Fatal("db file not exist", zap.String("path", dp))
	}

	hash, err := getHash(dp)
	if err != nil {
		lg.Fatal("failed to get hash", zap.Error(err))
	}
	fmt.Printf("db path: %s\nHash: %d\n", dp, hash)
}

func getHash(dbPath string) (hash uint32, err error) {
	b := backend.NewDefaultBackend(zap.NewNop(), dbPath)
	return b.Hash(schema.DefaultIgnores)
}
