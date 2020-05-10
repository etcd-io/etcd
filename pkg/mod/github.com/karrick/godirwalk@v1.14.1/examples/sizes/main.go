/*
 * sizes
 *
 * Walks a file system hierarchy and prints sizes of file system objects,
 * recursively printing sizes of directories.
 */
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karrick/godirwalk"
)

var progname = filepath.Base(os.Args[0])

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s dir1 [dir2 [dir3...]]\n", progname)
		os.Exit(2)
	}

	for _, arg := range os.Args[1:] {
		if err := sizes(arg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
		}
	}
}

func sizes(osDirname string) error {
	sizes := newSizesStack()

	return godirwalk.Walk(osDirname, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if de.IsDir() {
				sizes.EnterDirectory()
				return nil
			}

			st, err := os.Stat(osPathname)
			if err != nil {
				return err
			}

			size := st.Size()
			sizes.Accumulate(size)

			_, err = fmt.Printf("%s % 12d %s\n", st.Mode(), size, osPathname)
			return err
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
			return godirwalk.SkipNode
		},
		PostChildrenCallback: func(osPathname string, de *godirwalk.Dirent) error {
			size := sizes.LeaveDirectory()
			sizes.Accumulate(size) // add this directory's size to parent directory.

			st, err := os.Stat(osPathname)

			switch err {
			case nil:
				_, err = fmt.Printf("%s % 12d %s\n", st.Mode(), size, osPathname)
			default:
				// ignore the error and just show the mode type
				_, err = fmt.Printf("%s % 12d %s\n", de.ModeType(), size, osPathname)
			}
			return err
		},
	})
}

// sizesStack encapsulates operations on stack of directory sizes, with similar
// but slightly modified LIFO semantics to push and pop on a regular stack.
type sizesStack struct {
	sizes []int64 // stack of sizes
	top   int     // index of top of stack
}

func newSizesStack() *sizesStack {
	// Initialize with dummy value at top of stack to eliminate special cases.
	return &sizesStack{sizes: make([]int64, 1, 32)}
}

func (s *sizesStack) EnterDirectory() {
	s.sizes = append(s.sizes, 0)
	s.top++
}

func (s *sizesStack) LeaveDirectory() (i int64) {
	i, s.sizes = s.sizes[s.top], s.sizes[:s.top]
	s.top--
	return i
}

func (s *sizesStack) Accumulate(i int64) {
	s.sizes[s.top] += i
}
