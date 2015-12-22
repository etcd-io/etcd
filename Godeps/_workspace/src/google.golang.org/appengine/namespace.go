// Copyright 2012 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package appengine

import (
	"fmt"
	"regexp"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/golang/protobuf/proto"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/appengine/internal"
)

// Namespace returns a replacement context that operates within the given namespace.
func Namespace(c context.Context, namespace string) (context.Context, error) {
	if !validNamespace.MatchString(namespace) {
		return nil, fmt.Errorf("appengine: namespace %q does not match /%s/", namespace, validNamespace)
	}
	n := &namespacedContext{
		namespace: namespace,
	}
	return internal.WithNamespace(internal.WithCallOverride(c, n.call), namespace), nil
}

// validNamespace matches valid namespace names.
var validNamespace = regexp.MustCompile(`^[0-9A-Za-z._-]{0,100}$`)

// namespacedContext wraps a Context to support namespaces.
type namespacedContext struct {
	namespace string
}

func (n *namespacedContext) call(ctx context.Context, service, method string, in, out proto.Message) error {
	// Apply any namespace mods.
	if mod, ok := internal.NamespaceMods[service]; ok {
		mod(in, n.namespace)
	}
	return internal.Call(ctx, service, method, in, out)
}
