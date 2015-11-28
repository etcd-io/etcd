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

package datastore_test

import (
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/oauth2"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/oauth2/google"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/cloud/datastore"
)

// TODO(jbd): Remove after Go 1.4.
// Related to https://codereview.appspot.com/107320046
func TestA(t *testing.T) {}

func Example_auth() context.Context {
	// Initialize an authorized context with Google Developers Console
	// JSON key. Read the google package examples to learn more about
	// different authorization flows you can use.
	// http://godoc.org/golang.org/x/oauth2/google
	jsonKey, err := ioutil.ReadFile("/path/to/json/keyfile.json")
	if err != nil {
		log.Fatal(err)
	}
	conf, err := google.JWTConfigFromJSON(
		jsonKey,
		datastore.ScopeDatastore,
		datastore.ScopeUserEmail,
	)
	if err != nil {
		log.Fatal(err)
	}
	ctx := cloud.NewContext("project-id", conf.Client(oauth2.NoContext))
	// Use the context (see other examples)
	return ctx
}

func ExampleGet() {
	ctx := Example_auth()

	type Article struct {
		Title       string
		Description string
		Body        string `datastore:",noindex"`
		Author      *datastore.Key
		PublishedAt time.Time
	}
	key := datastore.NewKey(ctx, "Article", "articled1", 0, nil)
	article := &Article{}
	if err := datastore.Get(ctx, key, article); err != nil {
		log.Fatal(err)
	}
}

func ExamplePut() {
	ctx := Example_auth()

	type Article struct {
		Title       string
		Description string
		Body        string `datastore:",noindex"`
		Author      *datastore.Key
		PublishedAt time.Time
	}
	newKey := datastore.NewIncompleteKey(ctx, "Article", nil)
	_, err := datastore.Put(ctx, newKey, &Article{
		Title:       "The title of the article",
		Description: "The description of the article...",
		Body:        "...",
		Author:      datastore.NewKey(ctx, "Author", "jbd", 0, nil),
		PublishedAt: time.Now(),
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleDelete() {
	ctx := Example_auth()

	key := datastore.NewKey(ctx, "Article", "articled1", 0, nil)
	if err := datastore.Delete(ctx, key); err != nil {
		log.Fatal(err)
	}
}

type Post struct {
	Title       string
	PublishedAt time.Time
	Comments    int
}

func ExampleGetMulti() {
	ctx := Example_auth()

	keys := []*datastore.Key{
		datastore.NewKey(ctx, "Post", "post1", 0, nil),
		datastore.NewKey(ctx, "Post", "post2", 0, nil),
		datastore.NewKey(ctx, "Post", "post3", 0, nil),
	}
	posts := make([]Post, 3)
	if err := datastore.GetMulti(ctx, keys, posts); err != nil {
		log.Println(err)
	}
}

func ExamplePutMulti_slice() {
	ctx := Example_auth()

	keys := []*datastore.Key{
		datastore.NewKey(ctx, "Post", "post1", 0, nil),
		datastore.NewKey(ctx, "Post", "post2", 0, nil),
	}

	// PutMulti with a Post slice.
	posts := []*Post{
		{Title: "Post 1", PublishedAt: time.Now()},
		{Title: "Post 2", PublishedAt: time.Now()},
	}
	if _, err := datastore.PutMulti(ctx, keys, posts); err != nil {
		log.Fatal(err)
	}
}

func ExamplePutMulti_interfaceSlice() {
	ctx := Example_auth()

	keys := []*datastore.Key{
		datastore.NewKey(ctx, "Post", "post1", 0, nil),
		datastore.NewKey(ctx, "Post", "post2", 0, nil),
	}

	// PutMulti with an empty interface slice.
	posts := []interface{}{
		&Post{Title: "Post 1", PublishedAt: time.Now()},
		&Post{Title: "Post 2", PublishedAt: time.Now()},
	}
	if _, err := datastore.PutMulti(ctx, keys, posts); err != nil {
		log.Fatal(err)
	}
}

func ExampleQuery() {
	ctx := Example_auth()

	// Count the number of the post entities.
	n, err := datastore.NewQuery("Post").Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("There are %d posts.", n)

	// List the posts published since yesterday.
	yesterday := time.Now().Add(-24 * time.Hour)
	it := datastore.NewQuery("Post").Filter("PublishedAt >", yesterday).Run(ctx)
	// Use the iterator.
	_ = it

	// Order the posts by the number of comments they have recieved.
	datastore.NewQuery("Post").Order("-Comments")

	// Start listing from an offset and limit the results.
	datastore.NewQuery("Post").Offset(20).Limit(10)
}

func ExampleTransaction() {
	ctx := Example_auth()
	const retries = 3

	// Increment a counter.
	// See https://cloud.google.com/appengine/articles/sharding_counters for
	// a more scalable solution.
	type Counter struct {
		Count int
	}

	key := datastore.NewKey(ctx, "counter", "CounterA", 0, nil)

	for i := 0; i < retries; i++ {
		tx, err := datastore.NewTransaction(ctx)
		if err != nil {
			break
		}

		var c Counter
		if err := tx.Get(key, &c); err != nil && err != datastore.ErrNoSuchEntity {
			break
		}
		c.Count++
		if _, err := tx.Put(key, &c); err != nil {
			break
		}

		// Attempt to commit the transaction. If there's a conflict, try again.
		if _, err := tx.Commit(); err != datastore.ErrConcurrentTransaction {
			break
		}
	}

}
