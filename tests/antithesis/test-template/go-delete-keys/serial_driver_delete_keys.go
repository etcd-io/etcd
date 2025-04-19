package main

import (
	"context"
	"log"
	"os"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Connect to an etcd node
func Connect() *clientv3.Client {
	hosts := [][]string{[]string{"etcd0:2379"}, []string{"etcd1:2379"}, []string{"etcd2:2379"}}

	// Randomly choose one host to connect to (just for simplicity)
	host := hosts[0] // You can implement a random choice function here if needed

	// Create an etcd client
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   host,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
		os.Exit(1)
	}
	return cli
}

// This function will:
// 1. Get all keys
// 2. Select half of the keys received
// 3. Attempt to delete the keys selected
// 4. Check that the keys were deleted
func DeleteKeys() {
	ctx := context.Background()

	// Connect to an etcd node
	cli := Connect()

	// Get all keys with the prefix
	resp, err := cli.Get(ctx, "", clientv3.WithPrefix())
	if err != nil {
		log.Printf("Client failed to get all keys: %v", err)
		os.Exit(0)
	}

	// Choose half of the keys to delete
	var keys []string
	for _, k := range resp.Kvs {
		keys = append(keys, string(k.Key))
	}
	half := len(keys) / 2
	halfKeys := keys[:half]

	// Connect to a new etcd node
	cli = Connect()

	// Delete half of the selected keys
	var deletedKeys []string
	for _, k := range halfKeys {
		_, err := cli.Delete(ctx, k)
		if err != nil {
			log.Printf("Failed to delete key %s: %v", k, err)
		} else {
			log.Printf("Successfully deleted key %v", k)
			deletedKeys = append(deletedKeys, k)
		}
	}
	cli.Close()

	// Connect to a new etcd node
	cli = Connect()

	// Check if the deleted keys exist
	for _, k := range deletedKeys {
		resp, err := cli.Get(ctx, k)
		if err != nil {
			log.Printf("Client failed to get key %s: %v", k, err)
			continue
		}

		// Assert that the key was deleted correctly (now simply check if the key count is 0)
		if resp.Count != 0 {
			log.Printf("Key %s was not deleted correctly", k)
		} else {
			log.Printf("Key %s was deleted correctly", k)
		}
	}
	cli.Close()

	log.Printf("Completion of key deleting check")
}

func main() {
	DeleteKeys()
}
