// Copyright 2016 The etcd Authors
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

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	rootCommand = &cobra.Command{
		Use:   "etcd-dump-db",
		Short: "etcd-dump-db inspects etcd db files.",
	}
	listBucketCommand = &cobra.Command{
		Use:   "list-bucket [data dir or db file path]",
		Short: "bucket lists all buckets.",
		Run:   listBucketCommandFunc,
	}
	iterateBucketCommand = &cobra.Command{
		Use:   "iterate-bucket [data dir or db file path] [bucket name]",
		Short: "iterate-bucket lists key-value pairs in reverse order.",
		Run:   iterateBucketCommandFunc,
	}
	getHashCommand = &cobra.Command{
		Use:   "hash [data dir or db file path]",
		Short: "hash computes the hash of db file.",
		Run:   getHashCommandFunc,
	}
)

var flockTimeout time.Duration
var iterateBucketLimit uint64
var iterateBucketDecode bool

func init() {
	rootCommand.PersistentFlags().DurationVar(&flockTimeout, "timeout", 10*time.Second, "time to wait to obtain a file lock on db file, 0 to block indefinitely")
	iterateBucketCommand.PersistentFlags().Uint64Var(&iterateBucketLimit, "limit", 0, "max number of key-value pairs to iterate (0< to iterate all)")
	iterateBucketCommand.PersistentFlags().BoolVar(&iterateBucketDecode, "decode", false, "true to decode Protocol Buffer encoded data")

	rootCommand.AddCommand(listBucketCommand)
	rootCommand.AddCommand(iterateBucketCommand)
	rootCommand.AddCommand(getHashCommand)
}

func main() {
	if err := rootCommand.Execute(); err != nil {
		fmt.Fprintln(os.Stdout, err)
		os.Exit(1)
	}
}

func listBucketCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		log.Fatalf("Must provide at least 1 argument (got %v)", args)
	}
	dp := args[0]
	if !strings.HasSuffix(dp, "db") {
		dp = filepath.Join(snapDir(dp), "db")
	}
	if !existFileOrDir(dp) {
		log.Fatalf("%q does not exist", dp)
	}

	bts, err := getBuckets(dp)
	if err != nil {
		log.Fatal(err)
	}
	for _, b := range bts {
		fmt.Println(b)
	}
}

func iterateBucketCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		log.Fatalf("Must provide 2 arguments (got %v)", args)
	}
	dp := args[0]
	if !strings.HasSuffix(dp, "db") {
		dp = filepath.Join(snapDir(dp), "db")
	}
	if !existFileOrDir(dp) {
		log.Fatalf("%q does not exist", dp)
	}
	bucket := args[1]
	err := iterateBucket(dp, bucket, iterateBucketLimit, iterateBucketDecode)
	if err != nil {
		log.Fatal(err)
	}
}

func getHashCommandFunc(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		log.Fatalf("Must provide at least 1 argument (got %v)", args)
	}
	dp := args[0]
	if !strings.HasSuffix(dp, "db") {
		dp = filepath.Join(snapDir(dp), "db")
	}
	if !existFileOrDir(dp) {
		log.Fatalf("%q does not exist", dp)
	}

	hash, err := getHash(dp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("db path: %s\nHash: %d\n", dp, hash)
}
