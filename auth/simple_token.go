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

package auth

// CAUTION: This randum number based token mechanism is only for testing purpose.
// JWT based mechanism will be added in the near future.

import (
	"crypto/rand"
	"math/big"
	"sync"
)

const (
	letters                  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	defaultSimpleTokenLength = 16
)

var (
	simpleTokensMu sync.RWMutex
	simpleTokens   map[string]string // token -> username
)

func init() {
	simpleTokens = make(map[string]string)
}

func genSimpleToken() (string, error) {
	ret := make([]byte, defaultSimpleTokenLength)

	for i := 0; i < defaultSimpleTokenLength; i++ {
		bInt, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}

		ret[i] = letters[bInt.Int64()]
	}

	return string(ret), nil
}

func genSimpleTokenForUser(username string) (string, error) {
	var token string
	var err error

	for {
		// generating random numbers in RSM would't a good idea
		token, err = genSimpleToken()
		if err != nil {
			return "", err
		}

		if _, ok := simpleTokens[token]; !ok {
			break
		}
	}

	simpleTokensMu.Lock()
	simpleTokens[token] = username
	simpleTokensMu.Unlock()

	return token, nil
}
