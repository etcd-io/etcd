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

package storage

import (
	"encoding/base64"
	"io"
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	raw "google.golang.org/api/storage/v1"
)

// Bucket represents a Google Cloud Storage bucket.
type Bucket struct {
	// Name is the name of the bucket.
	Name string

	// ACL is the list of access control rules on the bucket.
	ACL []ACLRule

	// DefaultObjectACL is the list of access controls to
	// apply to new objects when no object ACL is provided.
	DefaultObjectACL []ACLRule

	// Location is the location of the bucket. It defaults to "US".
	Location string

	// Metageneration is the metadata generation of the bucket.
	// Read-only.
	Metageneration int64

	// StorageClass is the storage class of the bucket. This defines
	// how objects in the bucket are stored and determines the SLA
	// and the cost of storage. Typical values are "STANDARD" and
	// "DURABLE_REDUCED_AVAILABILITY". Defaults to "STANDARD".
	StorageClass string

	// Created is the creation time of the bucket.
	// Read-only.
	Created time.Time
}

func newBucket(b *raw.Bucket) *Bucket {
	if b == nil {
		return nil
	}
	bucket := &Bucket{
		Name:           b.Name,
		Location:       b.Location,
		Metageneration: b.Metageneration,
		StorageClass:   b.StorageClass,
		Created:        convertTime(b.TimeCreated),
	}
	acl := make([]ACLRule, len(b.Acl))
	for i, rule := range b.Acl {
		acl[i] = ACLRule{
			Entity: ACLEntity(rule.Entity),
			Role:   ACLRole(rule.Role),
		}
	}
	bucket.ACL = acl
	objACL := make([]ACLRule, len(b.DefaultObjectAcl))
	for i, rule := range b.DefaultObjectAcl {
		objACL[i] = ACLRule{
			Entity: ACLEntity(rule.Entity),
			Role:   ACLRole(rule.Role),
		}
	}
	bucket.DefaultObjectACL = objACL
	return bucket
}

// ObjectAttrs is the user-editable object attributes.
type ObjectAttrs struct {
	// Name is the name of the object.
	Name string

	// ContentType is the MIME type of the object's content.
	// Optional.
	ContentType string

	// ContentLanguage is the optional RFC 1766 Content-Language of
	// the object's content sent in response headers.
	ContentLanguage string

	// ContentEncoding is the optional Content-Encoding of the object
	// sent it the response headers.
	ContentEncoding string

	// CacheControl is the optional Cache-Control header of the object
	// sent in the response headers.
	CacheControl string

	// ACL is the list of access control rules for the object.
	// Optional. If nil or empty, existing ACL rules are preserved.
	ACL []ACLRule

	// Metadata represents user-provided metadata, in key/value pairs.
	// It can be nil if the current metadata values needs to preserved.
	Metadata map[string]string
}

func (o ObjectAttrs) toRawObject(bucket string) *raw.Object {
	var acl []*raw.ObjectAccessControl
	if len(o.ACL) > 0 {
		acl = make([]*raw.ObjectAccessControl, len(o.ACL))
		for i, rule := range o.ACL {
			acl[i] = &raw.ObjectAccessControl{
				Entity: string(rule.Entity),
				Role:   string(rule.Role),
			}
		}
	}
	return &raw.Object{
		Bucket:          bucket,
		Name:            o.Name,
		ContentType:     o.ContentType,
		ContentEncoding: o.ContentEncoding,
		ContentLanguage: o.ContentLanguage,
		CacheControl:    o.CacheControl,
		Acl:             acl,
		Metadata:        o.Metadata,
	}
}

// Object represents a Google Cloud Storage (GCS) object.
type Object struct {
	// Bucket is the name of the bucket containing this GCS object.
	Bucket string

	// Name is the name of the object within the bucket.
	Name string

	// ContentType is the MIME type of the object's content.
	ContentType string

	// ContentLanguage is the content language of the object's content.
	ContentLanguage string

	// CacheControl is the Cache-Control header to be sent in the response
	// headers when serving the object data.
	CacheControl string

	// ACL is the list of access control rules for the object.
	ACL []ACLRule

	// Owner is the owner of the object.
	//
	// If non-zero, it is in the form of "user-<userId>".
	Owner string

	// Size is the length of the object's content.
	Size int64

	// ContentEncoding is the encoding of the object's content.
	ContentEncoding string

	// MD5 is the MD5 hash of the object's content.
	MD5 []byte

	// CRC32C is the CRC32 checksum of the object's content using
	// the Castagnoli93 polynomial.
	CRC32C uint32

	// MediaLink is an URL to the object's content.
	MediaLink string

	// Metadata represents user-provided metadata, in key/value pairs.
	// It can be nil if no metadata is provided.
	Metadata map[string]string

	// Generation is the generation number of the object's content.
	Generation int64

	// MetaGeneration is the version of the metadata for this
	// object at this generation. This field is used for preconditions
	// and for detecting changes in metadata. A metageneration number
	// is only meaningful in the context of a particular generation
	// of a particular object.
	MetaGeneration int64

	// StorageClass is the storage class of the bucket.
	// This value defines how objects in the bucket are stored and
	// determines the SLA and the cost of storage. Typical values are
	// "STANDARD" and "DURABLE_REDUCED_AVAILABILITY".
	// It defaults to "STANDARD".
	StorageClass string

	// Deleted is the time the object was deleted.
	// If not deleted, it is the zero value.
	Deleted time.Time

	// Updated is the creation or modification time of the object.
	// For buckets with versioning enabled, changing an object's
	// metadata does not change this property.
	Updated time.Time
}

// convertTime converts a time in RFC3339 format to time.Time.
// If any error occurs in parsing, the zero-value time.Time is silently returned.
func convertTime(t string) time.Time {
	var r time.Time
	if t != "" {
		r, _ = time.Parse(time.RFC3339, t)
	}
	return r
}

func newObject(o *raw.Object) *Object {
	if o == nil {
		return nil
	}
	acl := make([]ACLRule, len(o.Acl))
	for i, rule := range o.Acl {
		acl[i] = ACLRule{
			Entity: ACLEntity(rule.Entity),
			Role:   ACLRole(rule.Role),
		}
	}
	owner := ""
	if o.Owner != nil {
		owner = o.Owner.Entity
	}
	md5, _ := base64.StdEncoding.DecodeString(o.Md5Hash)
	var crc32c uint32
	d, err := base64.StdEncoding.DecodeString(o.Crc32c)
	if err == nil && len(d) == 4 {
		crc32c = uint32(d[0])<<24 + uint32(d[1])<<16 + uint32(d[2])<<8 + uint32(d[3])
	}
	return &Object{
		Bucket:          o.Bucket,
		Name:            o.Name,
		ContentType:     o.ContentType,
		ContentLanguage: o.ContentLanguage,
		CacheControl:    o.CacheControl,
		ACL:             acl,
		Owner:           owner,
		ContentEncoding: o.ContentEncoding,
		Size:            int64(o.Size),
		MD5:             md5,
		CRC32C:          crc32c,
		MediaLink:       o.MediaLink,
		Metadata:        o.Metadata,
		Generation:      o.Generation,
		MetaGeneration:  o.Metageneration,
		StorageClass:    o.StorageClass,
		Deleted:         convertTime(o.TimeDeleted),
		Updated:         convertTime(o.Updated),
	}
}

// Query represents a query to filter objects from a bucket.
type Query struct {
	// Delimiter returns results in a directory-like fashion.
	// Results will contain only objects whose names, aside from the
	// prefix, do not contain delimiter. Objects whose names,
	// aside from the prefix, contain delimiter will have their name,
	// truncated after the delimiter, returned in prefixes.
	// Duplicate prefixes are omitted.
	// Optional.
	Delimiter string

	// Prefix is the prefix filter to query objects
	// whose names begin with this prefix.
	// Optional.
	Prefix string

	// Versions indicates whether multiple versions of the same
	// object will be included in the results.
	Versions bool

	// Cursor is a previously-returned page token
	// representing part of the larger set of results to view.
	// Optional.
	Cursor string

	// MaxResults is the maximum number of items plus prefixes
	// to return. As duplicate prefixes are omitted,
	// fewer total results may be returned than requested.
	// The default page limit is used if it is negative or zero.
	MaxResults int
}

// Objects represents a list of objects returned from
// a bucket look-p request and a query to retrieve more
// objects from the next pages.
type Objects struct {
	// Results represent a list of object results.
	Results []*Object

	// Next is the continuation query to retrieve more
	// results with the same filtering criteria. If there
	// are no more results to retrieve, it is nil.
	Next *Query

	// Prefixes represents prefixes of objects
	// matching-but-not-listed up to and including
	// the requested delimiter.
	Prefixes []string
}

// contentTyper implements ContentTyper to enable an
// io.ReadCloser to specify its MIME type.
type contentTyper struct {
	io.Reader
	t string
}

func (c *contentTyper) ContentType() string {
	return c.t
}

// A Writer writes a Cloud Storage object.
type Writer struct {
	// ObjectAttrs are optional attributes to set on the object. Any attributes
	// must be initialized before the first Write call. Nil or zero-valued
	// attributes are ignored.
	ObjectAttrs

	ctx    context.Context
	bucket string
	name   string

	once sync.Once

	opened bool
	r      io.Reader
	pw     *io.PipeWriter

	donec chan struct{} // closed after err and obj are set.
	err   error
	obj   *Object
}

func (w *Writer) open() {
	attrs := w.ObjectAttrs
	// Always set the name, otherwise the backend
	// rejects the request and responds with an HTTP 400.
	if attrs.Name == "" {
		attrs.Name = w.name
	}
	pr, pw := io.Pipe()
	w.r = &contentTyper{pr, attrs.ContentType}
	w.pw = pw
	w.opened = true

	go func() {
		resp, err := rawService(w.ctx).Objects.Insert(
			w.bucket, attrs.toRawObject(w.bucket)).Media(w.r).Projection("full").Do()
		w.err = err
		if err == nil {
			w.obj = newObject(resp)
		} else {
			pr.CloseWithError(w.err)
		}
		close(w.donec)
	}()
}

// Write appends to w.
func (w *Writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if !w.opened {
		w.open()
	}
	return w.pw.Write(p)
}

// Close completes the write operation and flushes any buffered data.
// If Close doesn't return an error, metadata about the written object
// can be retrieved by calling Object.
func (w *Writer) Close() error {
	if !w.opened {
		w.open()
	}
	if err := w.pw.Close(); err != nil {
		return err
	}
	<-w.donec
	return w.err
}

// Object returns metadata about a successfully-written object.
// It's only valid to call it after Close returns nil.
func (w *Writer) Object() *Object {
	return w.obj
}
