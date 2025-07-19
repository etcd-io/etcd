package offline

import (
	"fmt"
	"log"
	"os"
	"sort"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

var keyBucketName = []byte("key")

type keyItem struct {
	key      string
	revCount int
}

type keyStats []keyItem

func (ks keyStats) Len() int           { return len(ks) }
func (ks keyStats) Less(i, j int) bool { return ks[i].revCount > ks[j].revCount }
func (ks keyStats) Swap(i, j int)      { ks[i], ks[j] = ks[j], ks[i] }

func AnalyzeOffline(dataDir string) {
	log.Println("etcd diagnosis performs offline analysis...")

	dbPath := ToBackendFileName(dataDir)
	if !fileExists(dbPath) {
		log.Printf("%s does not exist", dbPath)
		return
	}

	db, err := bolt.Open(dbPath, 0600, &bolt.Options{ReadOnly: true})
	if err != nil {
		log.Fatalf("Failed to open db: %s, error: %v", dbPath, err)
	}

	keyMap := make(map[string][]BucketKey)

	_ = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(keyBucketName)

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			rev := BytesToBucketKey(k)

			kv := &mvccpb.KeyValue{}
			if uerr := kv.Unmarshal(v); uerr != nil {
				log.Printf("Failed to unmarshal key: %s, error: %v", string(k), uerr)
			}

			key := string(kv.Key)
			if revList, ok := keyMap[key]; ok {
				revList = append(revList, rev)
				keyMap[key] = revList
			} else {
				keyMap[key] = []BucketKey{rev}
			}
		}

		return nil
	})

	printStats(keyMap)
}

func printStats(keyMap map[string][]BucketKey) {
	var allKeyStats []keyItem
	for k, v := range keyMap {
		allKeyStats = append(allKeyStats, keyItem{k, len(v)})
	}
	sort.Sort(keyStats(allKeyStats))

	fmt.Println("All key stats:")
	for _, k := range allKeyStats {
		fmt.Printf("%s: %d\n", k.key, k.revCount)
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Fatalf("Error checking file existence of %s: %v", filename, err)
	return false
}
