package godirwalk

import "os"

func isDirectoryOrSymlinkToDirectory(de *Dirent, osPathname string) (bool, error) {
	if de.IsDir() {
		return true, nil
	}
	return isSymlinkToDirectory(de, osPathname)
}

func isSymlinkToDirectory(de *Dirent, osPathname string) (bool, error) {
	if !de.IsSymlink() {
		return false, nil
	}
	// Need to resolve the symbolic link's referent in order to respond.
	info, err := os.Stat(osPathname)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}
