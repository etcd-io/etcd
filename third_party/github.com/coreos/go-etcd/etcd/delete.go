package etcd

// Delete deletes the given key.
// When recursive set to false If the key points to a
// directory, the method will fail.
// When recursive set to true, if the key points to a file,
// the file will be deleted.  If the key points
// to a directory, then everything under the directory, including
// all child directories, will be deleted.
func (c *Client) Delete(key string, recursive bool) (*Response, error) {
	raw, err := c.DeleteRaw(key, recursive)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

func (c *Client) DeleteRaw(key string, recursive bool) (*RawResponse, error) {
	ops := options{
		"recursive": recursive,
	}

	return c.delete(key, ops)
}
