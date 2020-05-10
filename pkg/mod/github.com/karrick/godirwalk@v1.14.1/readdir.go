package godirwalk

// ReadDirents returns a sortable slice of pointers to Dirent structures, each
// representing the file system name and mode type for one of the immediate
// descendant of the specified directory. If the specified directory is a
// symbolic link, it will be resolved.
//
// The second parameter was an optional scratch buffer, but is no longer used
// because ReadDirents invokes Scanner to enumerate the contents of the
// directory.
//
//    children, err := godirwalk.ReadDirents(osDirname, nil)
//    if err != nil {
//        return nil, errors.Wrap(err, "cannot get list of directory children")
//    }
//    sort.Sort(children)
//    for _, child := range children {
//        fmt.Printf("%s %s\n", child.ModeType, child.Name)
//    }
func ReadDirents(osDirname string, _ []byte) (Dirents, error) {
	var entries Dirents
	scanner, err := NewScanner(osDirname)
	if err != nil {
		return nil, err
	}
	for scanner.Scan() {
		if dirent, err := scanner.Dirent(); err == nil {
			entries = append(entries, dirent)
		}
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// ReadDirnames returns a slice of strings, representing the immediate
// descendants of the specified directory. If the specified directory is a
// symbolic link, it will be resolved.
//
// The second parameter was an optional scratch buffer, but is no longer used
// because ReadDirents invokes Scanner to enumerate the contents of the
// directory.
//
// Note that this function, depending on operating system, may or may not invoke
// the ReadDirents function, in order to prepare the list of immediate
// descendants. Therefore, if your program needs both the names and the file
// system mode types of descendants, it will always be faster to invoke
// ReadDirents directly, rather than calling this function, then looping over
// the results and calling os.Stat or os.LStat for each entry.
//
//    children, err := godirwalk.ReadDirnames(osDirname, nil)
//    if err != nil {
//        return nil, errors.Wrap(err, "cannot get list of directory children")
//    }
//    sort.Strings(children)
//    for _, child := range children {
//        fmt.Printf("%s\n", child)
//    }
func ReadDirnames(osDirname string, _ []byte) ([]string, error) {
	var entries []string
	scanner, err := NewScanner(osDirname)
	if err != nil {
		return nil, err
	}
	for scanner.Scan() {
		entries = append(entries, scanner.Name())
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
