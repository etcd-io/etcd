package etcd

// GetDir gets the all contents under the given key.
// If the key points to a file, the file is returned.
// If the key points to a directory, everything under it is returnd,
// including all contents under all child directories.
func (c *Client) GetAll(key string, sort bool) (*Response, error) {
	return c.get(key, options{
		"recursive": true,
		"sorted":    sort,
	})
}

// Get gets the file or directory associated with the given key.
// If the key points to a directory, files and directories under
// it will be returned in sorted or unsorted order, depending on
// the sort flag.  Note that contents under child directories
// will not be returned.  To get those contents, use GetAll.
func (c *Client) Get(key string, sort bool) (*Response, error) {
	return c.get(key, options{
		"sorted": sort,
	})
}
