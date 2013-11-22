package etcd

// DeleteAll deletes everything under the given key.  If the key
// points to a file, the file will be deleted.  If the key points
// to a directory, then everything under the directory, include
// all child directories, will be deleted.
func (c *Client) DeleteAll(key string) (*Response, error) {
	return c.delete(key, options{
		"recursive": true,
	})
}

// Delete deletes the given key.  If the key points to a
// directory, the method will fail.
func (c *Client) Delete(key string) (*Response, error) {
	return c.delete(key, nil)
}
