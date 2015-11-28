// Copyright 2014 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// This example only works on Managed VMs.
// +build !appengine

package main

import (
	"html/template"
	"net/http"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine/log"
)

var initTime = time.Now()

func main() {
	http.HandleFunc("/", handle)
	appengine.Main()
}

func handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx := appengine.NewContext(r)
	log.Infof(ctx, "Serving the front page.")

	tmpl.Execute(w, time.Since(initTime))
}

var tmpl = template.Must(template.New("front").Parse(`
<html><body>

<p>
Hello, World! 세상아 안녕!
</p>

<p>
This instance has been running for <em>{{.}}</em>.
</p>

</body></html>
`))
