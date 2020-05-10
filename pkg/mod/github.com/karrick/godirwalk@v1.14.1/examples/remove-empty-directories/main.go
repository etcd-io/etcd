/*
 * remove-empty-directories
 *
 * Walks a file system hierarchy and removes all directories with no children.
 */
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karrick/godirwalk"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s dir1 [dir2 [dir3...]]\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	var count, total int
	var err error

	for _, arg := range os.Args[1:] {
		count, err = pruneEmptyDirectories(arg)
		total += count
		if err != nil {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "Removed %d empty directories\n", total)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func pruneEmptyDirectories(osDirname string) (int, error) {
	var count int

	err := godirwalk.Walk(osDirname, &godirwalk.Options{
		Unsorted: true,
		Callback: func(_ string, _ *godirwalk.Dirent) error {
			// no-op while diving in; all the fun happens in PostChildrenCallback
			return nil
		},
		PostChildrenCallback: func(osPathname string, _ *godirwalk.Dirent) error {
			s, err := godirwalk.NewScanner(osPathname)
			if err != nil {
				return err
			}

			// Attempt to read only the first directory entry. Remember that
			// Scan skips both "." and ".." entries.
			hasAtLeastOneChild := s.Scan()

			// If error reading from directory, wrap up and return.
			if err := s.Err(); err != nil {
				return err
			}

			if hasAtLeastOneChild {
				return nil // do not remove directory with at least one child
			}
			if osPathname == osDirname {
				return nil // do not remove directory that was provided top-level directory
			}

			err = os.Remove(osPathname)
			if err == nil {
				count++
			}
			return err
		},
	})

	return count, err
}
