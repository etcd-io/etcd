// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package storage contains a Google Cloud Storage client.
package storage

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/internal"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"google.golang.org/api/googleapi"
	raw "google.golang.org/api/storage/v1"
)

var (
	ErrBucketNotExist = errors.New("storage: bucket doesn't exist")
	ErrObjectNotExist = errors.New("storage: object doesn't exist")
)

const (
	// ScopeFullControl grants permissions to manage your
	// data and permissions in Google Cloud Storage.
	ScopeFullControl = raw.DevstorageFull_controlScope

	// ScopeReadOnly grants permissions to
	// view your data in Google Cloud Storage.
	ScopeReadOnly = raw.DevstorageRead_onlyScope

	// ScopeReadWrite grants permissions to manage your
	// data in Google Cloud Storage.
	ScopeReadWrite = raw.DevstorageRead_writeScope
)

// TODO(jbd): Add storage.buckets.list.
// TODO(jbd): Add storage.buckets.insert.
// TODO(jbd): Add storage.buckets.update.
// TODO(jbd): Add storage.buckets.delete.

// TODO(jbd): Add storage.objects.watch.

// BucketInfo returns the metadata for the specified bucket.
func BucketInfo(ctx context.Context, name string) (*Bucket, error) {
	resp, err := rawService(ctx).Buckets.Get(name).Projection("full").Do()
	if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
		return nil, ErrBucketNotExist
	}
	if err != nil {
		return nil, err
	}
	return newBucket(resp), nil
}

// ListObjects lists objects from the bucket. You can specify a query
// to filter the results. If q is nil, no filtering is applied.
func ListObjects(ctx context.Context, bucket string, q *Query) (*Objects, error) {
	c := rawService(ctx).Objects.List(bucket)
	c.Projection("full")
	if q != nil {
		c.Delimiter(q.Delimiter)
		c.Prefix(q.Prefix)
		c.Versions(q.Versions)
		c.PageToken(q.Cursor)
		if q.MaxResults > 0 {
			c.MaxResults(int64(q.MaxResults))
		}
	}
	resp, err := c.Do()
	if err != nil {
		return nil, err
	}
	objects := &Objects{
		Results:  make([]*Object, len(resp.Items)),
		Prefixes: make([]string, len(resp.Prefixes)),
	}
	for i, item := range resp.Items {
		objects.Results[i] = newObject(item)
	}
	for i, prefix := range resp.Prefixes {
		objects.Prefixes[i] = prefix
	}
	if resp.NextPageToken != "" {
		next := Query{}
		if q != nil {
			// keep the other filtering
			// criteria if there is a query
			next = *q
		}
		next.Cursor = resp.NextPageToken
		objects.Next = &next
	}
	return objects, nil
}

// SignedURLOptions allows you to restrict the access to the signed URL.
type SignedURLOptions struct {
	// GoogleAccessID represents the authorizer of the signed URL generation.
	// It is typically the Google service account client email address from
	// the Google Developers Console in the form of "xxx@developer.gserviceaccount.com".
	// Required.
	GoogleAccessID string

	// PrivateKey is the Google service account private key. It is obtainable
	// from the Google Developers Console.
	// At https://console.developers.google.com/project/<your-project-id>/apiui/credential,
	// create a service account client ID or reuse one of your existing service account
	// credentials. Click on the "Generate new P12 key" to generate and download
	// a new private key. Once you download the P12 file, use the following command
	// to convert it into a PEM file.
	//
	//    $ openssl pkcs12 -in key.p12 -passin pass:notasecret -out key.pem -nodes
	//
	// Provide the contents of the PEM file as a byte slice.
	// Required.
	PrivateKey []byte

	// Method is the HTTP method to be used with the signed URL.
	// Signed URLs can be used with GET, HEAD, PUT, and DELETE requests.
	// Required.
	Method string

	// Expires is the expiration time on the signed URL. It must be
	// a datetime in the future.
	// Required.
	Expires time.Time

	// ContentType is the content type header the client must provide
	// to use the generated signed URL.
	// Optional.
	ContentType string

	// Headers is a list of extention headers the client must provide
	// in order to use the generated signed URL.
	// Optional.
	Headers []string

	// MD5 is the base64 encoded MD5 checksum of the file.
	// If provided, the client should provide the exact value on the request
	// header in order to use the signed URL.
	// Optional.
	MD5 []byte
}

// SignedURL returns a URL for the specified object. Signed URLs allow
// the users access to a restricted resource for a limited time without having a
// Google account or signing in. For more information about the signed
// URLs, see https://cloud.google.com/storage/docs/accesscontrol#Signed-URLs.
func SignedURL(bucket, name string, opts *SignedURLOptions) (string, error) {
	if opts.GoogleAccessID == "" || opts.PrivateKey == nil {
		return "", errors.New("storage: missing required credentials to generate a signed URL")
	}
	if opts.Method == "" {
		return "", errors.New("storage: missing required method option")
	}
	if opts.Expires.IsZero() {
		return "", errors.New("storage: missing required expires option")
	}
	key, err := parseKey(opts.PrivateKey)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n", opts.Method)
	fmt.Fprintf(h, "%s\n", opts.MD5)
	fmt.Fprintf(h, "%s\n", opts.ContentType)
	fmt.Fprintf(h, "%d\n", opts.Expires.Unix())
	fmt.Fprintf(h, "%s", strings.Join(opts.Headers, "\n"))
	fmt.Fprintf(h, "/%s/%s", bucket, name)
	b, err := rsa.SignPKCS1v15(
		rand.Reader,
		key,
		crypto.SHA256,
		h.Sum(nil),
	)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(b)
	u, err := url.Parse(fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, name))
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("GoogleAccessId", opts.GoogleAccessID)
	q.Set("Expires", fmt.Sprintf("%d", opts.Expires.Unix()))
	q.Set("Signature", string(encoded))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// StatObject returns meta information about the specified object.
func StatObject(ctx context.Context, bucket, name string) (*Object, error) {
	o, err := rawService(ctx).Objects.Get(bucket, name).Projection("full").Do()
	if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
		return nil, ErrObjectNotExist
	}
	if err != nil {
		return nil, err
	}
	return newObject(o), nil
}

// UpdateAttrs updates an object with the provided attributes.
// All zero-value attributes are ignored.
func UpdateAttrs(ctx context.Context, bucket, name string, attrs ObjectAttrs) (*Object, error) {
	o, err := rawService(ctx).Objects.Patch(bucket, name, attrs.toRawObject(bucket)).Projection("full").Do()
	if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusNotFound {
		return nil, ErrObjectNotExist
	}
	if err != nil {
		return nil, err
	}
	return newObject(o), nil
}

// DeleteObject deletes the specified object.
func DeleteObject(ctx context.Context, bucket, name string) error {
	return rawService(ctx).Objects.Delete(bucket, name).Do()
}

// CopyObject copies the source object to the destination.
// The copied object's attributes are overwritten by those given.
func CopyObject(ctx context.Context, bucket, name string, destBucket string, attrs ObjectAttrs) (*Object, error) {
	destName := name
	if attrs.Name != "" {
		destName = attrs.Name
	}
	o, err := rawService(ctx).Objects.Copy(
		bucket, name, destBucket, destName, attrs.toRawObject(destBucket)).Projection("full").Do()
	if err != nil {
		return nil, err
	}
	return newObject(o), nil
}

// NewReader creates a new io.ReadCloser to read the contents
// of the object.
func NewReader(ctx context.Context, bucket, name string) (io.ReadCloser, error) {
	hc := internal.HTTPClient(ctx)
	res, err := hc.Get(fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, name))
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, ErrObjectNotExist
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		res.Body.Close()
		return res.Body, fmt.Errorf("storage: can't read object %v/%v, status code: %v", bucket, name, res.Status)
	}
	return res.Body, nil
}

// NewWriter returns a storage Writer that writes to the GCS object
// identified by the specified name.
// If such an object doesn't exist, it creates one.
// Attributes can be set on the object by modifying the returned Writer's
// ObjectAttrs field before the first call to Write. The name parameter to this
// function is ignored if the Name field of the ObjectAttrs field is set to a
// non-empty string.
//
// It is the caller's responsibility to call Close when writing is done.
//
// The object is not available and any previous object with the same
// name is not replaced on Cloud Storage until Close is called.
func NewWriter(ctx context.Context, bucket, name string) *Writer {
	return &Writer{
		ctx:    ctx,
		bucket: bucket,
		name:   name,
		donec:  make(chan struct{}),
	}
}

func rawService(ctx context.Context) *raw.Service {
	return internal.Service(ctx, "storage", func(hc *http.Client) interface{} {
		svc, _ := raw.New(hc)
		return svc
	}).(*raw.Service)
}

// parseKey converts the binary contents of a private key file
// to an *rsa.PrivateKey. It detects whether the private key is in a
// PEM container or not. If so, it extracts the the private key
// from PEM container before conversion. It only supports PEM
// containers with no passphrase.
func parseKey(key []byte) (*rsa.PrivateKey, error) {
	if block, _ := pem.Decode(key); block != nil {
		key = block.Bytes
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(key)
	if err != nil {
		parsedKey, err = x509.ParsePKCS1PrivateKey(key)
		if err != nil {
			return nil, err
		}
	}
	parsed, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("oauth2: private key is invalid")
	}
	return parsed, nil
}
