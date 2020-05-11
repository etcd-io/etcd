// Copyright 2016 Michal Witkowski. All Rights Reserved.
// See LICENSE for licensing terms.

package grpc_retry

import (
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/util/backoffutils"
)

// BackoffLinear is very simple: it waits for a fixed period of time between calls.
func BackoffLinear(waitBetween time.Duration) BackoffFunc {
	return func(attempt uint) time.Duration {
		return waitBetween
	}
}

// BackoffLinearWithJitter waits a set period of time, allowing for jitter (fractional adjustment).
//
// For example waitBetween=1s and jitter=0.10 can generate waits between 900ms and 1100ms.
func BackoffLinearWithJitter(waitBetween time.Duration, jitterFraction float64) BackoffFunc {
	return func(attempt uint) time.Duration {
		return backoffutils.JitterUp(waitBetween, jitterFraction)
	}
}
