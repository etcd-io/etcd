// Manual code for validation tests.

package testpb

import (
	"errors"
	"math"
)

func (x *PingRequest) Validate(bool) error {
	if x.SleepTimeMs > 10000 {
		return errors.New("cannot sleep for more than 10s")
	}
	return nil
}

func (x *PingErrorRequest) Validate() error {
	if x.SleepTimeMs > 10000 {
		return errors.New("cannot sleep for more than 10s")
	}
	return nil
}

func (x *PingListRequest) Validate(bool) error {
	if x.SleepTimeMs > 10000 {
		return errors.New("cannot sleep for more than 10s")
	}
	return nil
}

func (x *PingStreamRequest) Validate(bool) error {
	if x.SleepTimeMs > 10000 {
		return errors.New("cannot sleep for more than 10s")
	}
	return nil
}

// Validate implements the legacy validation interface from protoc-gen-validate.
func (x *PingResponse) Validate() error {
	if x.Counter > math.MaxInt16 {
		return errors.New("ping allocation exceeded")
	}
	return nil
}

// ValidateAll implements the new ValidateAll interface from protoc-gen-validate.
func (x *PingResponse) ValidateAll() error {
	if x.Counter > math.MaxInt16 {
		return errors.New("ping allocation exceeded")
	}
	return nil
}

var (
	GoodPing       = &PingRequest{Value: "something", SleepTimeMs: 9999}
	GoodPingError  = &PingErrorRequest{Value: "something", SleepTimeMs: 9999}
	GoodPingList   = &PingListRequest{Value: "something", SleepTimeMs: 9999}
	GoodPingStream = &PingStreamRequest{Value: "something", SleepTimeMs: 9999}

	BadPing       = &PingRequest{Value: "something", SleepTimeMs: 10001}
	BadPingError  = &PingErrorRequest{Value: "something", SleepTimeMs: 10001}
	BadPingList   = &PingListRequest{Value: "something", SleepTimeMs: 10001}
	BadPingStream = &PingStreamRequest{Value: "something", SleepTimeMs: 10001}

	GoodPingResponse = &PingResponse{Counter: 100}
	BadPingResponse  = &PingResponse{Counter: math.MaxInt16 + 1}
)
