// +build !windows

package godirwalk

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

// MinimumScratchBufferSize specifies the minimum size of the scratch buffer
// that Walk, ReadDirents, ReadDirnames, and Scandir will use when reading file
// entries from the operating system. It is initialized to the result from
// calling `os.Getpagesize()` during program startup.
var MinimumScratchBufferSize = os.Getpagesize()

// Scanner is an iterator to enumerate the contents of a directory.
type Scanner struct {
	scratchBuffer []byte // read directory bytes from file system into this buffer
	workBuffer    []byte // points into scratchBuffer, from which we chunk out directory entries
	osDirname     string
	childName     string
	err           error    // err is the error associated with scanning directory
	statErr       error    // statErr is any error return while attempting to stat an entry
	dh            *os.File // used to close directory after done reading
	de            *Dirent  // most recently decoded directory entry
	sde           *syscall.Dirent
	fd            int // file descriptor used to read entries from directory
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
		scratchBuffer: make([]byte, MinimumScratchBufferSize),
		osDirname:     osDirname,
		dh:            dh,
		fd:            int(dh.Fd()),
	}
	return scanner, nil
}

// Dirent returns the current directory entry while scanning a directory.
func (s *Scanner) Dirent() (*Dirent, error) {
	if s.de == nil {
		s.de = &Dirent{name: s.childName}
		s.de.modeType, s.statErr = modeTypeFromDirent(s.sde, s.osDirname, s.childName)
	}
	return s.de, s.statErr
}

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

	s.osDirname, s.childName = "", ""
	s.scratchBuffer, s.workBuffer = nil, nil
	s.statErr, s.de, s.sde = nil, nil, nil
	s.fd = 0
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
func (s *Scanner) Name() string { return s.childName }

// Scan potentially reads and then decodes the next directory entry from the
// file system.
//
// When it returns false, this releases resources used by the Scanner then
// returns any error associated with closing the file system directory resource.
func (s *Scanner) Scan() bool {
	if s.err != nil {
		return false
	}

	for {
		// When the work buffer has nothing remaining to decode, we need to load
		// more data from disk.
		if len(s.workBuffer) == 0 {
			n, err := syscall.ReadDirent(s.fd, s.scratchBuffer)
			if err != nil {
				s.done(err)
				return false
			}
			if n <= 0 { // end of directory
				s.done(io.EOF)
				return false
			}
			s.workBuffer = s.scratchBuffer[:n] // trim work buffer to number of bytes read
		}

		// Loop until we have a usable file system entry, or we run out of data
		// in the work buffer.
		for len(s.workBuffer) > 0 {
			s.sde = (*syscall.Dirent)(unsafe.Pointer(&s.workBuffer[0])) // point entry to first syscall.Dirent in buffer
			s.workBuffer = s.workBuffer[reclen(s.sde):]                 // advance buffer for next iteration through loop

			if inoFromDirent(s.sde) == 0 {
				continue // inode set to 0 indicates an entry that was marked as deleted
			}

			nameSlice := nameFromDirent(s.sde)
			nameLength := len(nameSlice)

			if nameLength == 0 || (nameSlice[0] == '.' && (nameLength == 1 || (nameLength == 2 && nameSlice[1] == '.'))) {
				continue
			}

			s.de = nil
			s.childName = string(nameSlice)
			return true
		}
		// No more data in the work buffer, so loop around in the outside loop
		// to fetch more data.
	}
}
