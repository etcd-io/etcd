package main

import (
	"context"
	"fmt"
	"hash/crc32"
	"os"

	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(-1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "corrupt-checker",
	Short: "",
	Long:  "",
	Run:   checkFunc,
}

var (
	endpoint string
	revision int64
)

func init() {
	rootCmd.Flags().StringVarP(&endpoint, "endpoint", "e", "127.0.0.1:2379", "etcd endpoint")
	rootCmd.Flags().Int64VarP(&revision, "revision", "r", 0, "Revision to check consistency of, 0 for latest")
}

func checkFunc(cmd *cobra.Command, args []string) {
	client, err := clientv3.New(clientv3.Config{Endpoints: []string{endpoint}})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	hasMore := true
	key := ""
	rangeEnd := clientv3.GetPrefixRangeEnd(key)

	h := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	h.Write([]byte("key"))
	for hasMore {
		resp, err := client.Get(context.Background(), key, clientv3.WithFromKey(), clientv3.WithRange(rangeEnd), clientv3.WithLimit(10000), clientv3.WithRev(revision))
		if err != nil {
			panic(err)
		}
		for _, kv := range resp.Kvs {
			h.Write(kv.Key)
			h.Write(kv.Value)
		}
		if len(resp.Kvs) > 0 {
			lastKey := resp.Kvs[len(resp.Kvs)-1].Key
			key = string(lastKey) + "\x00"
		}
		hasMore = resp.More
		if revision == 0 {
			revision = resp.Header.Revision
		}
	}
	fmt.Printf("checksum: %d\n", h.Sum32())
	fmt.Printf("revision: %d\n", revision)
}
