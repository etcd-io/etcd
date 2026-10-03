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
// and provides the same functions as math/rand so it can be a drop-in replacement
//
// The usages of math/rand currently found in etcd (listing only the first occurrence of each here)
// to find all, use `for f in $(rg -l 'math/rand"'); do echo $f; grep 'rand\.' $f; done`
//
// 1. out = append(out, rand.Int63n(max-min)+min)
// 2. r := rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
// 3. func shuffleEndpoints(r *rand.Rand, eps []string) []string {
// 4. n := rand.Intn(3) + 3
// 5. key := fmt.Sprintf("foo%d", rand.Int())
// 6. membersToChange := rand.Perm(len(clus.Procs))[:numberOfMembersToChange]
// 7. if _, err := rand.Read(v); err != nil {
// 8. multiplier := jitter * (rand.Float64()*2 - 1)
// 9. traceNum := rand.Int31()
// 10. randID = rand.Uint64()
// 11. rand.Shuffle(len(perm), func(i, j int) {
package rand

import (
	mrand "math/rand"

	antithesis "github.com/antithesishq/antithesis-sdk-go/random"
)

var (
	src = antithesis.Source()
	r   = mrand.New(src)
)

type Rand = mrand.Rand

func New(_ mrand.Source) *mrand.Rand {
	return r
}

func NewSource(_ int64) mrand.Source {
	return src
}

func Int63n(n int64) int64 {
	return r.Int63n(n)
}

func Uint64() uint64 {
	return r.Uint64()
}

func Intn(n int) int {
	return r.Intn(n)
}

func Int() int {
	return r.Int()
}

func Perm(n int) []int {
	return r.Perm(n)
}

func Read(p []byte) (n int, err error) {
	return r.Read(p)
}

func Float64() float64 {
	return r.Float64()
}

func Int31() int32 {
	return r.Int31()
}

func Shuffle(n int, swap func(i, j int)) {
	r.Shuffle(n, swap)
}
