package raft

import (
	"io"
	"os"
)

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

	n, err := f.Write(data)
	if n < len(data) {
		f.Close()
		return io.ErrShortWrite
	}

	err = f.Sync()
	if err != nil {
		return err
	}

	return f.Close()
}
