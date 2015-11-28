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

// Package gcsdemo is an example App Engine or Mananged VM app using the Google Cloud Storage API.
package gcsdemo

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/oauth2"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/oauth2/google"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine/file"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine/log"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine/urlfetch"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/storage"
)

// bucket is a local cache of the app's default bucket name.
var bucket string // or: var bucket = "<your-app-id>.appspot.com"

func init() {
	http.HandleFunc("/", handler)
}

// demo struct holds information needed to run the various demo functions.
type demo struct {
	c   context.Context
	w   http.ResponseWriter
	ctx context.Context
	// cleanUp is a list of filenames that need cleaning up at the end of the demo.
	cleanUp []string
	// failed indicates that one or more of the demo steps failed.
	failed bool
}

func (d *demo) errorf(format string, args ...interface{}) {
	d.failed = true
	log.Errorf(d.c, format, args...)
}

// handler is the main demo entry point that calls the GCS operations.
func handler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	c := appengine.NewContext(r)
	if bucket == "" {
		var err error
		if bucket, err = file.DefaultBucketName(c); err != nil {
			log.Errorf(c, "failed to get default GCS bucket name: %v", err)
			return
		}
	}
	hc := &http.Client{
		Transport: &oauth2.Transport{
			Source: google.AppEngineTokenSource(c, storage.ScopeFullControl),
			Base:   &urlfetch.Transport{Context: c},
		},
	}
	ctx := cloud.NewContext(appengine.AppID(c), hc)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Demo GCS Application running from Version: %v\n", appengine.VersionID(c))
	fmt.Fprintf(w, "Using bucket name: %v\n\n", bucket)

	d := &demo{
		c:   c,
		w:   w,
		ctx: ctx,
	}

	n := "demo-testfile-go"
	d.createFile(n)
	d.readFile(n)
	d.copyFile(n)
	d.statFile(n)
	d.createListFiles()
	d.listBucket()
	d.listBucketDirMode()
	d.defaultACL()
	d.putDefaultACLRule()
	d.deleteDefaultACLRule()
	d.bucketACL()
	d.putBucketACLRule()
	d.deleteBucketACLRule()
	d.acl(n)
	d.putACLRule(n)
	d.deleteACLRule(n)
	d.deleteFiles()

	if d.failed {
		io.WriteString(w, "\nDemo failed.\n")
	} else {
		io.WriteString(w, "\nDemo succeeded.\n")
	}
}

// createFile creates a file in Google Cloud Storage.
func (d *demo) createFile(fileName string) {
	fmt.Fprintf(d.w, "Creating file /%v/%v\n", bucket, fileName)

	wc := storage.NewWriter(d.ctx, bucket, fileName)
	wc.ContentType = "text/plain"
	wc.Metadata = map[string]string{
		"x-goog-meta-foo": "foo",
		"x-goog-meta-bar": "bar",
	}
	d.cleanUp = append(d.cleanUp, fileName)

	if _, err := wc.Write([]byte("abcde\n")); err != nil {
		d.errorf("createFile: unable to write data to bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	if _, err := wc.Write([]byte(strings.Repeat("f", 1024*4) + "\n")); err != nil {
		d.errorf("createFile: unable to write data to bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	if err := wc.Close(); err != nil {
		d.errorf("createFile: unable to close bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
}

// readFile reads the named file in Google Cloud Storage.
func (d *demo) readFile(fileName string) {
	io.WriteString(d.w, "\nAbbreviated file content (first line and last 1K):\n")

	rc, err := storage.NewReader(d.ctx, bucket, fileName)
	if err != nil {
		d.errorf("readFile: unable to open file from bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	defer rc.Close()
	slurp, err := ioutil.ReadAll(rc)
	if err != nil {
		d.errorf("readFile: unable to read data from bucket %q, file %q: %v", bucket, fileName, err)
		return
	}

	fmt.Fprintf(d.w, "%s\n", bytes.SplitN(slurp, []byte("\n"), 2)[0])
	if len(slurp) > 1024 {
		fmt.Fprintf(d.w, "...%s\n", slurp[len(slurp)-1024:])
	} else {
		fmt.Fprintf(d.w, "%s\n", slurp)
	}
}

// copyFile copies a file in Google Cloud Storage.
func (d *demo) copyFile(fileName string) {
	copyName := fileName + "-copy"
	fmt.Fprintf(d.w, "Copying file /%v/%v to /%v/%v:\n", bucket, fileName, bucket, copyName)

	attrs := storage.ObjectAttrs{
		Name:        copyName,
		ContentType: "text/plain",
		Metadata: map[string]string{
			"x-goog-meta-foo-copy": "foo-copy",
			"x-goog-meta-bar-copy": "bar-copy",
		},
	}
	obj, err := storage.CopyObject(d.ctx, bucket, fileName, bucket, attrs)
	if err != nil {
		d.errorf("copyFile: unable to copy /%v/%v to bucket %q, file %q: %v", bucket, fileName, bucket, copyName, err)
		return
	}
	d.cleanUp = append(d.cleanUp, copyName)

	d.dumpStats(obj)
}

func (d *demo) dumpStats(obj *storage.Object) {
	fmt.Fprintf(d.w, "(filename: /%v/%v, ", obj.Bucket, obj.Name)
	fmt.Fprintf(d.w, "ContentType: %q, ", obj.ContentType)
	fmt.Fprintf(d.w, "ACL: %#v, ", obj.ACL)
	fmt.Fprintf(d.w, "Owner: %v, ", obj.Owner)
	fmt.Fprintf(d.w, "ContentEncoding: %q, ", obj.ContentEncoding)
	fmt.Fprintf(d.w, "Size: %v, ", obj.Size)
	fmt.Fprintf(d.w, "MD5: %q, ", obj.MD5)
	fmt.Fprintf(d.w, "CRC32C: %q, ", obj.CRC32C)
	fmt.Fprintf(d.w, "Metadata: %#v, ", obj.Metadata)
	fmt.Fprintf(d.w, "MediaLink: %q, ", obj.MediaLink)
	fmt.Fprintf(d.w, "StorageClass: %q, ", obj.StorageClass)
	if !obj.Deleted.IsZero() {
		fmt.Fprintf(d.w, "Deleted: %v, ", obj.Deleted)
	}
	fmt.Fprintf(d.w, "Updated: %v)\n", obj.Updated)
}

// statFile reads the stats of the named file in Google Cloud Storage.
func (d *demo) statFile(fileName string) {
	io.WriteString(d.w, "\nFile stat:\n")

	obj, err := storage.StatObject(d.ctx, bucket, fileName)
	if err != nil {
		d.errorf("statFile: unable to stat file from bucket %q, file %q: %v", bucket, fileName, err)
		return
	}

	d.dumpStats(obj)
}

// createListFiles creates files that will be used by listBucket.
func (d *demo) createListFiles() {
	io.WriteString(d.w, "\nCreating more files for listbucket...\n")
	for _, n := range []string{"foo1", "foo2", "bar", "bar/1", "bar/2", "boo/"} {
		d.createFile(n)
	}
}

// listBucket lists the contents of a bucket in Google Cloud Storage.
func (d *demo) listBucket() {
	io.WriteString(d.w, "\nListbucket result:\n")

	query := &storage.Query{Prefix: "foo"}
	for query != nil {
		objs, err := storage.ListObjects(d.ctx, bucket, query)
		if err != nil {
			d.errorf("listBucket: unable to list bucket %q: %v", bucket, err)
			return
		}
		query = objs.Next

		for _, obj := range objs.Results {
			d.dumpStats(obj)
		}
	}
}

func (d *demo) listDir(name, indent string) {
	query := &storage.Query{Prefix: name, Delimiter: "/"}
	for query != nil {
		objs, err := storage.ListObjects(d.ctx, bucket, query)
		if err != nil {
			d.errorf("listBucketDirMode: unable to list bucket %q: %v", bucket, err)
			return
		}
		query = objs.Next

		for _, obj := range objs.Results {
			fmt.Fprint(d.w, indent)
			d.dumpStats(obj)
		}
		for _, dir := range objs.Prefixes {
			fmt.Fprintf(d.w, "%v(directory: /%v/%v)\n", indent, bucket, dir)
			d.listDir(dir, indent+"  ")
		}
	}
}

// listBucketDirMode lists the contents of a bucket in dir mode in Google Cloud Storage.
func (d *demo) listBucketDirMode() {
	io.WriteString(d.w, "\nListbucket directory mode result:\n")
	d.listDir("b", "")
}

// dumpDefaultACL prints out the default object ACL for this bucket.
func (d *demo) dumpDefaultACL() {
	acl, err := storage.DefaultACL(d.ctx, bucket)
	if err != nil {
		d.errorf("defaultACL: unable to list default object ACL for bucket %q: %v", bucket, err)
		return
	}
	for _, v := range acl {
		fmt.Fprintf(d.w, "Entity: %q, Role: %q\n", v.Entity, v.Role)
	}
}

// defaultACL displays the default object ACL for this bucket.
func (d *demo) defaultACL() {
	io.WriteString(d.w, "\nDefault object ACL:\n")
	d.dumpDefaultACL()
}

// putDefaultACLRule adds the "allUsers" default object ACL rule for this bucket.
func (d *demo) putDefaultACLRule() {
	io.WriteString(d.w, "\nPut Default object ACL Rule:\n")
	err := storage.PutDefaultACLRule(d.ctx, bucket, "allUsers", storage.RoleReader)
	if err != nil {
		d.errorf("putDefaultACLRule: unable to save default object ACL rule for bucket %q: %v", bucket, err)
		return
	}
	d.dumpDefaultACL()
}

// deleteDefaultACLRule deleted the "allUsers" default object ACL rule for this bucket.
func (d *demo) deleteDefaultACLRule() {
	io.WriteString(d.w, "\nDelete Default object ACL Rule:\n")
	err := storage.DeleteDefaultACLRule(d.ctx, bucket, "allUsers")
	if err != nil {
		d.errorf("deleteDefaultACLRule: unable to delete default object ACL rule for bucket %q: %v", bucket, err)
		return
	}
	d.dumpDefaultACL()
}

// dumpBucketACL prints out the bucket ACL.
func (d *demo) dumpBucketACL() {
	acl, err := storage.BucketACL(d.ctx, bucket)
	if err != nil {
		d.errorf("dumpBucketACL: unable to list bucket ACL for bucket %q: %v", bucket, err)
		return
	}
	for _, v := range acl {
		fmt.Fprintf(d.w, "Entity: %q, Role: %q\n", v.Entity, v.Role)
	}
}

// bucketACL displays the bucket ACL for this bucket.
func (d *demo) bucketACL() {
	io.WriteString(d.w, "\nBucket ACL:\n")
	d.dumpBucketACL()
}

// putBucketACLRule adds the "allUsers" bucket ACL rule for this bucket.
func (d *demo) putBucketACLRule() {
	io.WriteString(d.w, "\nPut Bucket ACL Rule:\n")
	err := storage.PutBucketACLRule(d.ctx, bucket, "allUsers", storage.RoleReader)
	if err != nil {
		d.errorf("putBucketACLRule: unable to save bucket ACL rule for bucket %q: %v", bucket, err)
		return
	}
	d.dumpBucketACL()
}

// deleteBucketACLRule deleted the "allUsers" bucket ACL rule for this bucket.
func (d *demo) deleteBucketACLRule() {
	io.WriteString(d.w, "\nDelete Bucket ACL Rule:\n")
	err := storage.DeleteBucketACLRule(d.ctx, bucket, "allUsers")
	if err != nil {
		d.errorf("deleteBucketACLRule: unable to delete bucket ACL rule for bucket %q: %v", bucket, err)
		return
	}
	d.dumpBucketACL()
}

// dumpACL prints out the ACL of the named file.
func (d *demo) dumpACL(fileName string) {
	acl, err := storage.ACL(d.ctx, bucket, fileName)
	if err != nil {
		d.errorf("dumpACL: unable to list file ACL for bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	for _, v := range acl {
		fmt.Fprintf(d.w, "Entity: %q, Role: %q\n", v.Entity, v.Role)
	}
}

// acl displays the ACL for the named file.
func (d *demo) acl(fileName string) {
	fmt.Fprintf(d.w, "\nACL for file %v:\n", fileName)
	d.dumpACL(fileName)
}

// putACLRule adds the "allUsers" ACL rule for the named file.
func (d *demo) putACLRule(fileName string) {
	fmt.Fprintf(d.w, "\nPut ACL rule for file %v:\n", fileName)
	err := storage.PutACLRule(d.ctx, bucket, fileName, "allUsers", storage.RoleReader)
	if err != nil {
		d.errorf("putACLRule: unable to save ACL rule for bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	d.dumpACL(fileName)
}

// deleteACLRule deleted the "allUsers" ACL rule for the named file.
func (d *demo) deleteACLRule(fileName string) {
	fmt.Fprintf(d.w, "\nDelete ACL rule for file %v:\n", fileName)
	err := storage.DeleteACLRule(d.ctx, bucket, fileName, "allUsers")
	if err != nil {
		d.errorf("deleteACLRule: unable to delete ACL rule for bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	d.dumpACL(fileName)
}

// deleteFiles deletes all the temporary files from a bucket created by this demo.
func (d *demo) deleteFiles() {
	io.WriteString(d.w, "\nDeleting files...\n")
	for _, v := range d.cleanUp {
		fmt.Fprintf(d.w, "Deleting file %v\n", v)
		if err := storage.DeleteObject(d.ctx, bucket, v); err != nil {
			d.errorf("deleteFiles: unable to delete bucket %q, file %q: %v", bucket, v, err)
			return
		}
	}
}
