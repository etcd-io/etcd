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

package clientv3

import (
	"context"
	"errors"
	"fmt"
)

// defaultChannelKey is reserved for the built-in gRPC channel used when a
// context carries no explicit channel key.
const defaultChannelKey = "default"

// ErrChannelKeyNotFound reports that a non-default channel key was not
// initialized.
var ErrChannelKeyNotFound = errors.New("etcdclient: channel key is not initialized")

// ErrReservedChannelKey reports that a caller used a reserved channel key.
var ErrReservedChannelKey = errors.New("etcdclient: channel key is reserved")

type channelKeyContextKey struct{}

// WithChannelKey returns a context that selects the client-side gRPC channel
// identified by key.
func WithChannelKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, channelKeyContextKey{}, key)
}

// ChannelKeyFromContext returns the explicit client-side gRPC channel key.
func ChannelKeyFromContext(ctx context.Context) string {
	key, _ := channelKeyFromContext(ctx)
	return key
}

func channelKeyForRequest(ctx context.Context) string {
	key := ChannelKeyFromContext(ctx)
	if key == "" || key == defaultChannelKey {
		return defaultChannelKey
	}
	return key
}

func channelKeyFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	key, ok := ctx.Value(channelKeyContextKey{}).(string)
	return key, ok
}

func validateChannelKeys(keys []string) error {
	for _, key := range keys {
		if err := validateChannelKey(key); err != nil {
			return err
		}
	}
	return nil
}

func validateChannelKey(key string) error {
	if key == "" || key == defaultChannelKey {
		return fmt.Errorf("%w: %q", ErrReservedChannelKey, key)
	}
	return nil
}
