package etcd

// SetDir sets the given key to a directory.
func (c *Client) SetDir(key string, ttl uint64) (*Response, error) {
	raw, err := c.RawSetDir(key, ttl)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

// UpdateDir updates the given key to a directory.  It succeeds only if the
// given key already exists.
func (c *Client) UpdateDir(key string, ttl uint64) (*Response, error) {
	raw, err := c.RawUpdateDir(key, ttl)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

// UpdateDir creates a directory under the given key.  It succeeds only if
// the given key does not yet exist.
func (c *Client) CreateDir(key string, ttl uint64) (*Response, error) {
	raw, err := c.RawCreateDir(key, ttl)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

// Set sets the given key to the given value.
func (c *Client) Set(key string, value string, ttl uint64) (*Response, error) {
	raw, err := c.RawSet(key, value, ttl)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

// Update updates the given key to the given value.  It succeeds only if the
// given key already exists.
func (c *Client) Update(key string, value string, ttl uint64) (*Response, error) {
	raw, err := c.RawUpdate(key, value, ttl)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

// Create creates a file with the given value under the given key.  It succeeds
// only if the given key does not yet exist.
func (c *Client) Create(key string, value string, ttl uint64) (*Response, error) {
	raw, err := c.RawCreate(key, value, ttl)

	if err != nil {
		return nil, err
	}

	return raw.toResponse()
}

func (c *Client) RawSetDir(key string, ttl uint64) (*RawResponse, error) {
	return c.put(key, "", ttl, nil)
}

func (c *Client) RawUpdateDir(key string, ttl uint64) (*RawResponse, error) {
	ops := options{
		"prevExist": true,
	}

	return c.put(key, "", ttl, ops)
}

func (c *Client) RawCreateDir(key string, ttl uint64) (*RawResponse, error) {
	ops := options{
		"prevExist": false,
	}

	return c.put(key, "", ttl, ops)
}

func (c *Client) RawSet(key string, value string, ttl uint64) (*RawResponse, error) {
	return c.put(key, value, ttl, nil)
}

func (c *Client) RawUpdate(key string, value string, ttl uint64) (*RawResponse, error) {
	ops := options{
		"prevExist": true,
	}

	return c.put(key, value, ttl, ops)
}

func (c *Client) RawCreate(key string, value string, ttl uint64) (*RawResponse, error) {
	ops := options{
		"prevExist": false,
	}

	return c.put(key, value, ttl, ops)
}
