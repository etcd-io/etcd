// Copyright 2017 The etcd Authors
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

import (
	"context"
	"testing"
)

const (
	jwtPubKey  = "../../integration/fixtures/server.crt"
	jwtPrivKey = "../../integration/fixtures/server.key.insecure"
)

func TestJWTInfo(t *testing.T) {
	opts := map[string]string{
		"pub-key":     jwtPubKey,
		"priv-key":    jwtPrivKey,
		"sign-method": "RS256",
	}
	jwt, err := newTokenProviderJWT(opts)
	if err != nil {
		t.Fatal(err)
	}
	token, aerr := jwt.assign(context.TODO(), "abc", 123)
	if aerr != nil {
		t.Fatal(err)
	}
	ai, ok := jwt.info(context.TODO(), token, 123)
	if !ok {
		t.Fatalf("failed to authenticate with token %s", token)
	}
	if ai.Revision != 123 {
		t.Fatalf("expected revision 123, got %d", ai.Revision)
	}
	ai, ok = jwt.info(context.TODO(), "aaa", 120)
	if ok || ai != nil {
		t.Fatalf("expected aaa to fail to authenticate, got %+v", ai)
	}
}

func TestJWTBad(t *testing.T) {
	opts := map[string]string{
		"pub-key":     jwtPubKey,
		"priv-key":    jwtPrivKey,
		"sign-method": "RS256",
	}
	// private key instead of public key
	opts["pub-key"] = jwtPrivKey
	if _, err := newTokenProviderJWT(opts); err == nil {
		t.Fatalf("expected failure on missing public key")
	}
	opts["pub-key"] = jwtPubKey

	// public key instead of private key
	opts["priv-key"] = jwtPubKey
	if _, err := newTokenProviderJWT(opts); err == nil {
		t.Fatalf("expected failure on missing public key")
	}
	opts["priv-key"] = jwtPrivKey

	// missing signing option
	delete(opts, "sign-method")
	if _, err := newTokenProviderJWT(opts); err == nil {
		t.Fatal("expected error on missing option")
	}
	opts["sign-method"] = "RS256"

	// bad file for pubkey
	opts["pub-key"] = "whatever"
	if _, err := newTokenProviderJWT(opts); err == nil {
		t.Fatalf("expected failure on missing public key")
	}
	opts["pub-key"] = jwtPubKey

	// bad file for private key
	opts["priv-key"] = "whatever"
	if _, err := newTokenProviderJWT(opts); err == nil {
		t.Fatalf("expeceted failure on missing private key")
	}
	opts["priv-key"] = jwtPrivKey
}
