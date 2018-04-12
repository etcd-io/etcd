// Copyright 2018 The etcd Authors
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

package tester

import (
	"math/rand"
	"time"

	"go.uber.org/zap"
)

func (clus *Cluster) shuffleCases() {
	rand.Seed(time.Now().UnixNano())
	offset := rand.Intn(1000)
	n := len(clus.cases)
	cp := coprime(n)

	css := make([]Case, n)
	for i := 0; i < n; i++ {
		css[i] = clus.cases[(cp*i+offset)%n]
	}
	clus.cases = css
	clus.lg.Info("shuffled test failure cases", zap.Int("total", n))
}

/*
x and y of GCD 1 are coprime to each other

x1 = ( coprime of n * idx1 + offset ) % n
x2 = ( coprime of n * idx2 + offset ) % n
(x2 - x1) = coprime of n * (idx2 - idx1) % n
          = (idx2 - idx1) = 1

Consecutive x's are guaranteed to be distinct
*/
func coprime(n int) int {
	coprime := 1
	for i := n / 2; i < n; i++ {
		if gcd(i, n) == 1 {
			coprime = i
			break
		}
	}
	return coprime
}

func gcd(x, y int) int {
	if y == 0 {
		return x
	}
	return gcd(y, x%y)
}
