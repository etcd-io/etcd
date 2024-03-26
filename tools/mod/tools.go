// Copyright 2016 The etcd Authors
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

//go:build tools

// This file implements that pattern:
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
// for etcd. Thanks to this file 'go mod tidy' does not removes dependencies.

package tools

import (
	_ "github.com/alexfalkowski/gocovmerge"
	_ "github.com/appscodelabs/license-bill-of-materials"
	_ "github.com/chzchzchz/goword"
	_ "github.com/cloudflare/cfssl/cmd/cfssl"
	_ "github.com/cloudflare/cfssl/cmd/cfssljson"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/google/addlicense"
	_ "github.com/google/yamlfmt/cmd/yamlfmt"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"
	_ "github.com/mikefarah/yq/v4"
	_ "go.etcd.io/gofail"
	_ "go.etcd.io/protodoc"
	_ "go.etcd.io/raft/v3"
	_ "gotest.tools/gotestsum"
	_ "gotest.tools/v3"
	_ "honnef.co/go/tools/cmd/staticcheck"
)
