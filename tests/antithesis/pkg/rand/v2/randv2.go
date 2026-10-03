// Copyright 2026 The etcd Authors
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

// Package rand is a wrapper of https://antithesis.com/docs/generated/sdk/golang/random/
// and provides the same functions as math/rand/v2
//
// The usages of math/rand/v2 currently found in etcd
//
//  1. server/etcdserver/txn/util_bench_test.go
//     rng := rand.New(rand.NewPCG(1, 2))
//  2. tests/robustness/validate/operations_test.go
//     rand.Shuffle(len(historyCopy), func(i, j int)
//  3. tests/antithesis/test-template/robustness/traffic/main.go
//     choice := rand.IntN(len(traffics))
package rand

import (
	mrandv1 "math/rand"
	mrandv2 "math/rand/v2"

	antithesis "github.com/antithesishq/antithesis-sdk-go/random"
)

type sourceV2Func func() uint64

func (f sourceV2Func) Uint64() uint64 {
	return f()
}

// antithesis doesn't provide a math/rand/v2 source, so this is
// needed for making a v1 source a v2.
var (
	v1Rand   = mrandv1.New(antithesis.Source())
	v2Source = sourceV2Func(v1Rand.Uint64)
	r        = mrandv2.New(v2Source)
)

func New(_ mrandv2.Source) *mrandv2.Rand {
	return r
}

func NewPCG(_, _ int) mrandv2.Source {
	return v2Source
}

func IntN(n int) int {
	return r.IntN(n)
}

func Shuffle(n int, swap func(i, j int)) {
	r.Shuffle(n, swap)
}
