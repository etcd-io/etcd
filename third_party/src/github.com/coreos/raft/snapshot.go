package raft

import (
	//"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
)

//------------------------------------------------------------------------------
//
// Typedefs
//
//------------------------------------------------------------------------------

// the in memory SnapShot struct
// TODO add cluster configuration
type Snapshot struct {
	LastIndex uint64 `json:"lastIndex"`
	LastTerm  uint64 `json:"lastTerm"`
	// cluster configuration.
	Peers []*Peer `json:"peers"`
	State []byte  `json:"state"`
	Path  string  `json:"path"`
}

// Save the snapshot to a file
func (ss *Snapshot) save() error {
	// Write machine state to temporary buffer.

	// open file
	file, err := os.OpenFile(ss.Path, os.O_CREATE|os.O_WRONLY, 0600)

	if err != nil {
		return err
	}

	defer file.Close()

	b, err := json.Marshal(ss)

	// Generate checksum.
	checksum := crc32.ChecksumIEEE(b)

	// Write snapshot with checksum.
	if _, err = fmt.Fprintf(file, "%08x\n", checksum); err != nil {
		return err
	}

	if _, err = file.Write(b); err != nil {
		return err
	}

	// force the change writing to disk
	file.Sync()
	return err
}

// remove the file of the snapshot
func (ss *Snapshot) remove() error {
	err := os.Remove(ss.Path)
	return err
}
