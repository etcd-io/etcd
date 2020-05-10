// +build windows

package godirwalk

import (
	"fmt"
	"io"
	"os"
)

// Scanner is an iterator to enumerate the contents of a directory.
type Scanner struct {
	osDirname string
	dh        *os.File // dh is handle to open directory
	de        *Dirent
	err       error // err is the error associated with scanning directory
}

// NewScanner returns a new directory Scanner that lazily enumerates the
// contents of a single directory.
//
//     scanner, err := godirwalk.NewScanner(dirname)
//     if err != nil {
//         fatal("cannot scan directory: %s", err)
//     }
//
//     for scanner.Scan() {
//         dirent, err := scanner.Dirent()
//         if err != nil {
//             warning("cannot get dirent: %s", err)
//             continue
//         }
//         name := dirent.Name()
//         if name == "break" {
//             break
//         }
//         if name == "continue" {
//             continue
//         }
//         fmt.Printf("%v %v\n", dirent.ModeType(), dirent.Name())
//     }
//     if err := scanner.Err(); err != nil {
//         fatal("cannot scan directory: %s", err)
//     }
func NewScanner(osDirname string) (*Scanner, error) {
	dh, err := os.Open(osDirname)
	if err != nil {
		return nil, err
	}
	scanner := &Scanner{
		osDirname: osDirname,
		dh:        dh,
	}
	return scanner, nil
}

// Dirent returns the current directory entry while scanning a directory.
func (s *Scanner) Dirent() (*Dirent, error) { return s.de, nil }

// done is called when directory scanner unable to continue, with either the
// triggering error, or nil when there are simply no more entries to read from
// the directory.
func (s *Scanner) done(err error) {
	if s.dh == nil {
		return
	}
	cerr := s.dh.Close()
	s.dh = nil

	if err == nil {
		s.err = cerr
	} else {
		s.err = err
	}

	s.osDirname = ""
	s.de = nil
}

// Err returns the error associated with scanning a directory.
func (s *Scanner) Err() error {
	s.done(s.err)
	if s.err == io.EOF {
		return nil
	}
	return s.err
}

// Name returns the name of the current directory entry while scanning a
// directory.
func (s *Scanner) Name() string { return s.de.name }

// Scan potentially reads and then decodes the next directory entry from the
// file system.
func (s *Scanner) Scan() bool {
	if s.err != nil {
		return false
	}

	fileinfos, err := s.dh.Readdir(1)
	if err != nil {
		s.err = err
		return false
	}

	if l := len(fileinfos); l != 1 {
		s.err = fmt.Errorf("expected a single entry rather than %d", l)
		return false
	}

	fi := fileinfos[0]
	s.de = &Dirent{
		name:     fi.Name(),
		modeType: fi.Mode() & os.ModeType,
	}
	return true
}
