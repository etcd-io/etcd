package raft

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
)

// Snapshot represents an in-memory representation of the current state of the system.
type Snapshot struct {
	LastIndex uint64 `json:"lastIndex"`
	LastTerm  uint64 `json:"lastTerm"`

	// Cluster configuration.
	Peers []*Peer `json:"peers"`
	State []byte  `json:"state"`
	Path  string  `json:"path"`
}

// save writes the snapshot to file.
func (ss *Snapshot) save() error {
	// Open the file for writing.
	file, err := os.OpenFile(ss.Path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Serialize to JSON.
	b, err := json.Marshal(ss)
	if err != nil {
		return err
	}

	// Generate checksum and write it to disk.
	checksum := crc32.ChecksumIEEE(b)
	if _, err = fmt.Fprintf(file, "%08x\n", checksum); err != nil {
		return err
	}

	// Write the snapshot to disk.
	if _, err = file.Write(b); err != nil {
		return err
	}

	// Ensure that the snapshot has been flushed to disk before continuing.
	if err := file.Sync(); err != nil {
		return err
	}

	return nil
}

// remove deletes the snapshot file.
func (ss *Snapshot) remove() error {
	if err := os.Remove(ss.Path); err != nil {
		return err
	}
	return nil
}
