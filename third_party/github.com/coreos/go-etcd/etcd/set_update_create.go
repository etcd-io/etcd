package etcd

// SetDir sets the given key to a directory.
func (c *Client) SetDir(key string, ttl uint64) (*Response, error) {
	return c.put(key, "", ttl, nil)
}

// UpdateDir updates the given key to a directory.  It succeeds only if the
// given key already exists.
func (c *Client) UpdateDir(key string, ttl uint64) (*Response, error) {
	return c.put(key, "", ttl, options{
		"prevExist": true,
	})
}

// UpdateDir creates a directory under the given key.  It succeeds only if
// the given key does not yet exist.
func (c *Client) CreateDir(key string, ttl uint64) (*Response, error) {
	return c.put(key, "", ttl, options{
		"prevExist": false,
	})
}

// Set sets the given key to the given value.
func (c *Client) Set(key string, value string, ttl uint64) (*Response, error) {
	return c.put(key, value, ttl, nil)
}

// Update updates the given key to the given value.  It succeeds only if the
// given key already exists.
func (c *Client) Update(key string, value string, ttl uint64) (*Response, error) {
	return c.put(key, value, ttl, options{
		"prevExist": true,
	})
}

// Create creates a file with the given value under the given key.  It succeeds
// only if the given key does not yet exist.
func (c *Client) Create(key string, value string, ttl uint64) (*Response, error) {
	return c.put(key, value, ttl, options{
		"prevExist": false,
	})
}
