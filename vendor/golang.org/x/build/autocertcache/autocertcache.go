// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package autocertcache contains autocert.Cache implementations
// for golang.org/x/crypto/autocert.
package autocertcache

import (
	"context"
	"io/ioutil"

	"cloud.google.com/go/storage"
	"golang.org/x/crypto/acme/autocert"
)

// NewGoogleCloudStorageCache returns an autocert.Cache storing its cache entries
// in the named Google Cloud Storage bucket. The implementation assumes that it
// owns the entire bucket namespace.
func NewGoogleCloudStorageCache(sc *storage.Client, bucket string) autocert.Cache {
	return &gcsAutocertCache{sc, bucket}
}

// gcsAutocertCache implements the
// golang.org/x/crypto/acme/autocert.Cache interface using a Google
// Cloud Storage bucket. It assumes that autocert gets to use the
// whole keyspace of the bucket. That is, don't reuse this bucket for
// other purposes.
type gcsAutocertCache struct {
	gcs    *storage.Client
	bucket string
}

func (c *gcsAutocertCache) Get(ctx context.Context, key string) ([]byte, error) {
	rd, err := c.gcs.Bucket(c.bucket).Object(key).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, autocert.ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}
	defer rd.Close()
	return ioutil.ReadAll(rd)
}

func (c *gcsAutocertCache) Put(ctx context.Context, key string, data []byte) error {
	wr := c.gcs.Bucket(c.bucket).Object(key).NewWriter(ctx)
	if _, err := wr.Write(data); err != nil {
		return err
	}
	return wr.Close()
}

func (c *gcsAutocertCache) Delete(ctx context.Context, key string) error {
	err := c.gcs.Bucket(c.bucket).Object(key).Delete(ctx)
	if err == storage.ErrObjectNotExist {
		return nil
	}
	return err
}
