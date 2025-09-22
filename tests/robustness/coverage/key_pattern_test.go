// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coverage_test

import (
	"strings"
	"testing"
)

func pattern(key string) (string, bool) {
	if key == "/registry/health" || key == "compact_rev_key" {
		return key, true
	}
	key, valid := strings.CutPrefix(key, "/registry")
	if !valid {
		return key, false
	}
	slash := ""
	if strings.HasSuffix(key, "/") {
		slash = "/"
	}
	parts := strings.Split(strings.Trim(key, "/"), "/")
	if len(parts[0]) == 0 {
		return key, false
	}

	apiResource, shift := resourceOrCRD(parts[0])
	parts = parts[shift:]

	switch len(parts) {
	case 0:
		// Listing all resources.
		return "/registry/" + apiResource + slash, true
	case 1:
		if slash == "" {
			// Get on a non-namespaced resource.
			return "/registry/" + apiResource + "/{name}", true
		}
		// Listing all resources in a namespace.
		return "/registry/" + apiResource + "/{namespace}" + slash, true
	case 2:
		if slash == "" {
			// Get on a namespaced resource.
			return "/registry/" + apiResource + "/{namespace}/{name}", true
		}
	}

	return key, false
}

func resourceOrCRD(s string) (string, int) {
	if strings.Contains(s, ".") {
		// Group name from the Custom Resource Definition.
		// Names without dots are technically valid DNS name, but against best practices:
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-subdomain-names
		// https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#customresourcedefinitionspec-v1-apiextensions-k8s-io
		return "{api-group}/{resource}", 2
	}
	switch s {
	case "services":
		// services have subresources specs and endpoints.
		return "services/{subresource}", 2
	case "masterleases":
		// masterleases are not a regular resource.
		return "masterleases", 1
	case "events":
		// events have watch cache disabled by default and use Etcd leases.
		return "events", 1
	}
	return "{resource}", 1
}

func TestPatternValid(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want string
	}{
		{
			key:  "compact_rev_key",
			want: "compact_rev_key",
		},
		{
			key:  "/registry/health",
			want: "/registry/health",
		},
		{
			key:  "/registry/resource/namespace/object",
			want: "/registry/{resource}/{namespace}/{name}",
		},
		{
			key:  "/registry/resource/namespace_list/",
			want: "/registry/{resource}/{namespace}/",
		},
		{
			key:  "/registry/services/specs/namespace/object",
			want: "/registry/services/{subresource}/{namespace}/{name}",
		},
		{
			key:  "/registry/services/endpoints",
			want: "/registry/services/{subresource}",
		},
		{
			key:  "/registry/services/specs/namespace_list/",
			want: "/registry/services/{subresource}/{namespace}/",
		},
		{
			key:  "/registry/resource/object_get",
			want: "/registry/{resource}/{name}",
		},
		{
			key:  "/registry/resource_rev",
			want: "/registry/{resource}",
		},
		{
			key:  "/registry/resource_list/",
			want: "/registry/{resource}/",
		},
		{
			key:  "/registry/crd.io/resource/namespace/object_get",
			want: "/registry/{api-group}/{resource}/{namespace}/{name}",
		},
		{
			key:  "/registry/crd.io/resource/namespace_list/",
			want: "/registry/{api-group}/{resource}/{namespace}/",
		},
		{
			key:  "/registry/crd.io/resource/object_get",
			want: "/registry/{api-group}/{resource}/{name}",
		},
		{
			key:  "/registry/crd.io/resource_rev",
			want: "/registry/{api-group}/{resource}",
		},
		{
			key:  "/registry/crd.io/resource_list/",
			want: "/registry/{api-group}/{resource}/",
		},
	} {
		name := strings.Trim(strings.ReplaceAll(tc.key, "/", "_"), "_")
		t.Run(name, func(t *testing.T) {
			got, valid := pattern(tc.key)
			if !valid {
				t.Fatalf("key=%q, want=valid, got=invalid", tc.key)
			}
			if got != tc.want {
				t.Fatalf("key=%q, want=%s, got=%s", tc.key, tc.want, got)
			}
		})
	}
}

func TestPatternInvalid(t *testing.T) {
	for _, tc := range []string{
		"/other-registry",
		"/registry//",
	} {
		name := strings.Trim(strings.ReplaceAll(tc, "/", "_"), "_")
		t.Run(name, func(t *testing.T) {
			got, valid := pattern(tc)
			if valid {
				t.Fatalf("key=%q, want=invalid, got=valid, pattern=%q", tc, got)
			}
		})
	}
}
