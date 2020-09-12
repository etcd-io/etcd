// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "qs IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package qos

import (
	"errors"

	"golang.org/x/time/rate"
)

const (
	RateLimiterTokenBucket = "TokenBucket"
	RateLimiterLeakyBucket = "LeakyBucket"
	RateLimiterMaxInflight = "MaxInflight"
)

var (
	ErrInvalidRateLimiterName = errors.New("qos: ratelimiter name is invalid")
)

type RateLimiter interface {
	// GetToken returns true if rate do not exceed. Otherwise,
	// it returns false.
	GetToken() bool
	// PutToken returns token to rate limiter.
	PutToken() bool
	// QPS returns QPS of rate limiter
	QPS() float32
	// Name returns Name of rate limiter
	Name() string
}

func NewRateLimiter(name string, qps uint64) (RateLimiter, error) {
	switch name {
	case RateLimiterTokenBucket:
		return newTokenBucketRateLimiter(float32(qps), int(qps)), nil
	case RateLimiterMaxInflight:
		return newMaxInflightRateLimiter(int(qps)), nil
	default:
		return nil, ErrInvalidRateLimiterName
	}
}

type tokenBucketRateLimiter struct {
	limiter *rate.Limiter
	qps     float32
	name    string
}

func newTokenBucketRateLimiter(qps float32, burst int) RateLimiter {
	limiter := rate.NewLimiter(rate.Limit(qps), burst)
	return &tokenBucketRateLimiter{
		limiter: limiter,
		qps:     qps,
		name:    RateLimiterTokenBucket,
	}
}

func (t *tokenBucketRateLimiter) GetToken() bool {
	return t.limiter.Allow()
}

func (t *tokenBucketRateLimiter) PutToken() bool {
	return true
}

func (t *tokenBucketRateLimiter) QPS() float32 {
	return t.qps
}

func (t *tokenBucketRateLimiter) Name() string {
	return t.name
}

type maxInflightRateLimiter struct {
	token       chan struct{}
	maxInflight int
	name        string
}

func newMaxInflightRateLimiter(qps int) RateLimiter {
	return &maxInflightRateLimiter{
		token:       make(chan struct{}, qps),
		maxInflight: qps,
		name:        RateLimiterMaxInflight,
	}
}

func (r *maxInflightRateLimiter) QPS() float32 {
	return float32(r.maxInflight)
}

func (r *maxInflightRateLimiter) Name() string {
	return r.name
}

func (r *maxInflightRateLimiter) GetToken() bool {
	select {
	case r.token <- struct{}{}:
		return true
	default:
		return false
	}
}

func (r *maxInflightRateLimiter) PutToken() bool {
	select {
	case <-r.token:
		return true
	default:
		return false
	}
}
