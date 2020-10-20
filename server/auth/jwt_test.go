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
	"fmt"
	"testing"

	"go.uber.org/zap"
)

const (
	jwtRSAPubKey  = "../tests/fixtures/server.crt"
	jwtRSAPrivKey = "../tests/fixtures/server.key.insecure"

	jwtECPubKey  = "../tests/fixtures/server-ecdsa.crt"
	jwtECPrivKey = "../tests/fixtures/server-ecdsa.key.insecure"
)

func TestJWTInfo(t *testing.T) {
	optsMap := map[string]map[string]string{
		"RSA-priv": {
			"priv-key":    jwtRSAPrivKey,
			"sign-method": "RS256",
			"ttl":         "1h",
		},
		"RSA": {
			"pub-key":     jwtRSAPubKey,
			"priv-key":    jwtRSAPrivKey,
			"sign-method": "RS256",
		},
		"RSAPSS-priv": {
			"priv-key":    jwtRSAPrivKey,
			"sign-method": "PS256",
		},
		"RSAPSS": {
			"pub-key":     jwtRSAPubKey,
			"priv-key":    jwtRSAPrivKey,
			"sign-method": "PS256",
		},
		"ECDSA-priv": {
			"priv-key":    jwtECPrivKey,
			"sign-method": "ES256",
		},
		"ECDSA": {
			"pub-key":     jwtECPubKey,
			"priv-key":    jwtECPrivKey,
			"sign-method": "ES256",
		},
		"HMAC": {
			"priv-key":    jwtECPrivKey, // any file, raw bytes used as shared secret
			"sign-method": "HS256",
		},
	}

	for k, opts := range optsMap {
		t.Run(k, func(tt *testing.T) {
			testJWTInfo(tt, opts)
		})
	}
}

func testJWTInfo(t *testing.T, opts map[string]string) {
	lg := zap.NewNop()
	jwt, err := newTokenProviderJWT(lg, opts)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.TODO()

	token, aerr := jwt.assign(ctx, "abc", 123)
	if aerr != nil {
		t.Fatalf("%#v", aerr)
	}
	ai, ok := jwt.info(ctx, token, 123)
	if !ok {
		t.Fatalf("failed to authenticate with token %s", token)
	}
	if ai.Revision != 123 {
		t.Fatalf("expected revision 123, got %d", ai.Revision)
	}
	ai, ok = jwt.info(ctx, "aaa", 120)
	if ok || ai != nil {
		t.Fatalf("expected aaa to fail to authenticate, got %+v", ai)
	}

	// test verify-only provider
	if opts["pub-key"] != "" && opts["priv-key"] != "" {
		t.Run("verify-only", func(t *testing.T) {
			newOpts := make(map[string]string, len(opts))
			for k, v := range opts {
				newOpts[k] = v
			}
			delete(newOpts, "priv-key")
			verify, err := newTokenProviderJWT(lg, newOpts)
			if err != nil {
				t.Fatal(err)
			}

			ai, ok := verify.info(ctx, token, 123)
			if !ok {
				t.Fatalf("failed to authenticate with token %s", token)
			}
			if ai.Revision != 123 {
				t.Fatalf("expected revision 123, got %d", ai.Revision)
			}
			ai, ok = verify.info(ctx, "aaa", 120)
			if ok || ai != nil {
				t.Fatalf("expected aaa to fail to authenticate, got %+v", ai)
			}

			_, aerr := verify.assign(ctx, "abc", 123)
			if aerr != ErrVerifyOnly {
				t.Fatalf("unexpected error when attempting to sign with public key: %v", aerr)
			}

		})
	}
}

func TestJWTBad(t *testing.T) {

	var badCases = map[string]map[string]string{
		"no options": {},
		"invalid method": {
			"sign-method": "invalid",
		},
		"rsa no key": {
			"sign-method": "RS256",
		},
		"invalid ttl": {
			"sign-method": "RS256",
			"ttl":         "forever",
		},
		"rsa invalid public key": {
			"sign-method": "RS256",
			"pub-key":     jwtRSAPrivKey,
			"priv-key":    jwtRSAPrivKey,
		},
		"rsa invalid private key": {
			"sign-method": "RS256",
			"pub-key":     jwtRSAPubKey,
			"priv-key":    jwtRSAPubKey,
		},
		"hmac no key": {
			"sign-method": "HS256",
		},
		"hmac pub key": {
			"sign-method": "HS256",
			"pub-key":     jwtRSAPubKey,
		},
		"missing public key file": {
			"sign-method": "HS256",
			"pub-key":     "missing-file",
		},
		"missing private key file": {
			"sign-method": "HS256",
			"priv-key":    "missing-file",
		},
		"ecdsa no key": {
			"sign-method": "ES256",
		},
		"ecdsa invalid public key": {
			"sign-method": "ES256",
			"pub-key":     jwtECPrivKey,
			"priv-key":    jwtECPrivKey,
		},
		"ecdsa invalid private key": {
			"sign-method": "ES256",
			"pub-key":     jwtECPubKey,
			"priv-key":    jwtECPubKey,
		},
	}

	lg := zap.NewNop()

	for k, v := range badCases {
		t.Run(k, func(t *testing.T) {
			_, err := newTokenProviderJWT(lg, v)
			if err == nil {
				t.Errorf("expected error for options %v", v)
			}
		})
	}
}

// testJWTOpts is useful for passing to NewTokenProvider which requires a string.
func testJWTOpts() string {
	return fmt.Sprintf("%s,pub-key=%s,priv-key=%s,sign-method=RS256", tokenTypeJWT, jwtRSAPubKey, jwtRSAPrivKey)
}
