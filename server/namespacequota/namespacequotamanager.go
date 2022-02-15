// Copyright 2021 The etcd Authors
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

package namespacequota

import (
	"errors"
)

type NamespaceQuotaEnforcement int8

const (
	DISABLED NamespaceQuotaEnforcement = 0
	SOFTMODE NamespaceQuotaEnforcement = 1
	HARDMODE NamespaceQuotaEnforcement = 2
)

var (
	// namespaceQuotaBucketName defines the name of bucket in the backend
	namespaceQuotaBucketName       = []byte("namespacequota")
	ErrNamespaceQuotaExceeded      = errors.New("namespace quota exceeded")
	ErrNamespaceQuotaNotFound      = errors.New("namespace quota not found")
	ErrNamespaceQuotaRestoreFailed = errors.New("namespace quota restore failed")
)

// NamespaceQuota represents a namespace quota
type NamespaceQuota struct {
	// Key represents the namespace key
	Key []byte
	// QuotaByteCount quota byte count represents the byte quota for respective Key
	QuotaByteCount uint64
	// QuotaKeyCount quota key count represents the quota key quota for respective Key
	QuotaKeyCount uint64
	// UsageByteCount represents the current usage of bytes for respective Key
	UsageByteCount uint64
	// UsageKeyCount represents the current usage of bytes for respective Key
	UsageKeyCount uint64
}
