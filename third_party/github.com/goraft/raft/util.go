package raft

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"time"
)

// uint64Slice implements sort interface
type uint64Slice []uint64

func (p uint64Slice) Len() int           { return len(p) }
func (p uint64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p uint64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// WriteFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise WriteFile truncates it before writing.
// This is copied from ioutil.WriteFile with the addition of a Sync call to
// ensure the data reaches the disk.
func writeFileSynced(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close() // Idempotent

	n, err := f.Write(data)
	if err == nil && n < len(data) {
		return io.ErrShortWrite
	} else if err != nil {
		return err
	}

	if err = f.Sync(); err != nil {
		return err
	}

	return f.Close()
}

// Waits for a random time between two durations and sends the current time on
// the returned channel.
func afterBetween(min time.Duration, max time.Duration) <-chan time.Time {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	d, delta := min, (max - min)
	if delta > 0 {
		d += time.Duration(rand.Int63n(int64(delta)))
	}
	return time.After(d)
}

// TODO(xiangli): Remove assertions when we reach version 1.0

// _assert will panic with a given formatted message if the given condition is false.
func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}
