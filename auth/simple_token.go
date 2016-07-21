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
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
)

const (
	letters                  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	defaultSimpleTokenLength = 16
	simpleTokenTTL           = 5 * time.Minute
	simpleTokenTTLResolution = 1 * time.Second
)

type simpleTokenTTLKeeper struct {
	tokens              map[string]time.Time
	addSimpleTokenCh    chan string
	resetSimpleTokenCh  chan string
	deleteSimpleTokenCh chan string
	stopCh              chan chan struct{}
	deleteTokenFunc     func(string)
}

func NewSimpleTokenTTLKeeper(deletefunc func(string)) *simpleTokenTTLKeeper {
	stk := &simpleTokenTTLKeeper{
		tokens:              make(map[string]time.Time),
		addSimpleTokenCh:    make(chan string, 1),
		resetSimpleTokenCh:  make(chan string, 1),
		deleteSimpleTokenCh: make(chan string, 1),
		stopCh:              make(chan chan struct{}),
		deleteTokenFunc:     deletefunc,
	}
	go stk.run()
	return stk
}

func (tm *simpleTokenTTLKeeper) stop() {
	waitCh := make(chan struct{})
	tm.stopCh <- waitCh
	<-waitCh
	close(tm.stopCh)
}

func (tm *simpleTokenTTLKeeper) addSimpleToken(token string) {
	tm.addSimpleTokenCh <- token
}

func (tm *simpleTokenTTLKeeper) resetSimpleToken(token string) {
	tm.resetSimpleTokenCh <- token
}

func (tm *simpleTokenTTLKeeper) deleteSimpleToken(token string) {
	tm.deleteSimpleTokenCh <- token
}
func (tm *simpleTokenTTLKeeper) run() {
	tokenTicker := time.NewTicker(simpleTokenTTLResolution)
	defer tokenTicker.Stop()
	for {
		select {
		case t := <-tm.addSimpleTokenCh:
			tm.tokens[t] = time.Now().Add(simpleTokenTTL)
		case t := <-tm.resetSimpleTokenCh:
			if _, ok := tm.tokens[t]; ok {
				tm.tokens[t] = time.Now().Add(simpleTokenTTL)
			}
		case t := <-tm.deleteSimpleTokenCh:
			delete(tm.tokens, t)
		case <-tokenTicker.C:
			nowtime := time.Now()
			for t, tokenendtime := range tm.tokens {
				if nowtime.After(tokenendtime) {
					tm.deleteTokenFunc(t)
					delete(tm.tokens, t)
				}
			}
		case waitCh := <-tm.stopCh:
			tm.tokens = make(map[string]time.Time)
			waitCh <- struct{}{}
			return
		}
	}
}

type tokenSimple struct {
	indexWaiter       func(uint64) <-chan struct{}
	simpleTokenKeeper *simpleTokenTTLKeeper
	simpleTokensMu    sync.RWMutex
	simpleTokens      map[string]string // token -> username
}

func (t *tokenSimple) genTokenPrefix() (string, error) {
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

func (t *tokenSimple) assignSimpleTokenToUser(username, token string) {
	t.simpleTokensMu.Lock()

	_, ok := t.simpleTokens[token]
	if ok {
		plog.Panicf("token %s is alredy used", token)
	}

	t.simpleTokens[token] = username
	t.simpleTokenKeeper.addSimpleToken(token)
	t.simpleTokensMu.Unlock()
}

func (t *tokenSimple) invalidateUser(username string) {
	t.simpleTokensMu.Lock()
	defer t.simpleTokensMu.Unlock()

	for token, name := range t.simpleTokens {
		if strings.Compare(name, username) == 0 {
			delete(t.simpleTokens, token)
			t.simpleTokenKeeper.deleteSimpleToken(token)
		}
	}
}

func newDeleterFunc(t *tokenSimple) func(string) {
	return func(tk string) {
		t.simpleTokensMu.Lock()
		defer t.simpleTokensMu.Unlock()
		if username, ok := t.simpleTokens[tk]; ok {
			plog.Infof("deleting token %s for user %s", tk, username)
			delete(t.simpleTokens, tk)
		}
	}
}

func (t *tokenSimple) enable() {
	t.simpleTokenKeeper = NewSimpleTokenTTLKeeper(newDeleterFunc(t))
}

func (t *tokenSimple) disable() {
	if t.simpleTokenKeeper != nil {
		t.simpleTokenKeeper.stop()
		t.simpleTokenKeeper = nil
	}

	t.simpleTokensMu.Lock()
	t.simpleTokens = make(map[string]string) // invalidate all tokens
	t.simpleTokensMu.Unlock()
}

func (t *tokenSimple) info(ctx context.Context, token string, revision uint64) (*AuthInfo, bool) {
	if !t.isValidSimpleToken(ctx, token) {
		return nil, false
	}

	t.simpleTokensMu.RLock()
	defer t.simpleTokensMu.RUnlock()
	username, ok := t.simpleTokens[token]
	if ok {
		t.simpleTokenKeeper.resetSimpleToken(token)
	}

	return &AuthInfo{Username: username, Revision: revision}, ok
}

func (t *tokenSimple) assign(ctx context.Context, username string, rev uint64) (string, error) {
	// rev isn't used in simple token, it is only used in JWT
	index := ctx.Value("index").(uint64)
	simpleToken := ctx.Value("simpleToken").(string)
	token := fmt.Sprintf("%s.%d", simpleToken, index)
	t.assignSimpleTokenToUser(username, token)

	return token, nil
}

func (t *tokenSimple) isValidSimpleToken(ctx context.Context, token string) bool {
	splitted := strings.Split(token, ".")
	if len(splitted) != 2 {
		return false
	}
	index, err := strconv.Atoi(splitted[1])
	if err != nil {
		return false
	}

	select {
	case <-t.indexWaiter(uint64(index)):
		return true
	case <-ctx.Done():
	}

	return false
}

func newTokenProviderSimple(indexWaiter func(uint64) <-chan struct{}) *tokenSimple {
	return &tokenSimple{
		simpleTokens: make(map[string]string),
		indexWaiter:  indexWaiter,
	}
}
