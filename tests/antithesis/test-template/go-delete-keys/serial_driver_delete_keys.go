package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/antithesishq/antithesis-sdk-go/assert"
	"github.com/antithesishq/antithesis-sdk-go/random"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func Connect() *clientv3.Client {
	// This function returns a client connection to an etcd node

	hosts := [][]string{[]string{"etcd0:2379"}, []string{"etcd1:2379"}, []string{"etcd2:2379"}}
	host := random.RandomChoice(hosts)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   host,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
		// Antithesis Assertion: client should always be able to connect to an etcd host
		assert.Unreachable("Client failed to connect to an etcd host", map[string]interface{}{"host": host, "error": err})
		os.Exit(1)
	}
	return cli
}

func DeleteKeys() {
	// This function will:
	// 1. Get all keys
	// 2. Select half of the keys received
	// 3. Attempt to delete the keys selected
	// 4. Check that the keys were deleted

	ctx := context.Background()

	// Connect to an etcd node
	cli := Connect()

	// Get all keys
	resp, err := cli.Get(ctx, "", clientv3.WithPrefix())

	// Antithesis Assertion: sometimes get with prefix requests are successful. A failed request is OK since we expect them to happen.
	assert.Sometimes(err == nil, "Client can make successful get all requests", map[string]interface{}{"error": err})
	cli.Close()

	if err != nil {
		log.Printf("Client failed to get all keys: %v", err)
		os.Exit(0)
	}

	// Choose half of the keys
	var keys []string
	for _, k := range resp.Kvs {
		keys = append(keys, string(k.Key))
	}
	half := len(keys) / 2
	halfKeys := keys[:half]

	// Connect to a new etcd node
	cli = Connect()

	// Delete half of the keys chosen
	var deletedKeys []string
	for _, k := range halfKeys {
		_, err := cli.Delete(ctx, k)
		// Antithesis Assertion: sometimes delete requests are successful. A failed request is OK since we expect them to happen.
		assert.Sometimes(err == nil, "Client can make successful delete requests", map[string]interface{}{"error": err})
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

	// Check to see if those keys were deleted / exist
	for _, k := range deletedKeys {
		resp, err := cli.Get(ctx, k)
		// Antithesis Assertion: sometimes get requests are successful. A failed request is OK since we expect them to happen.
		assert.Sometimes(err == nil, "Client can make successful get requests", map[string]interface{}{"error": err})
		if err != nil {
			log.Printf("Client failed to get key %s: %v", k, err)
			continue
		}
		// Antithesis Assertion: if we deleted a key, we should not get a value
		assert.Always(resp.Count == 0, "Key was deleted correctly", map[string]interface{}{"key": k})
	}
	cli.Close()

	assert.Reachable("Completion of a key deleting check", nil)
	log.Printf("Completion of a key deleting check")
}

func main() {
	DeleteKeys()
}