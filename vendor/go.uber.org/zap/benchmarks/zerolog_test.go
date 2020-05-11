// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package benchmarks

import (
	"io/ioutil"

	"github.com/rs/zerolog"
)

func newZerolog() zerolog.Logger {
	return zerolog.New(ioutil.Discard).With().Timestamp().Logger()
}

func newDisabledZerolog() zerolog.Logger {
	return newZerolog().Level(zerolog.Disabled)
}

func fakeZerologFields(e *zerolog.Event) *zerolog.Event {
	return e.
		Int("int", _tenInts[0]).
		Interface("ints", _tenInts).
		Str("string", _tenStrings[0]).
		Interface("strings", _tenStrings).
		Time("time", _tenTimes[0]).
		Interface("times", _tenTimes).
		Interface("user1", _oneUser).
		Interface("user2", _oneUser).
		Interface("users", _tenUsers).
		Err(errExample)
}

func fakeZerologContext(c zerolog.Context) zerolog.Context {
	return c.
		Int("int", _tenInts[0]).
		Interface("ints", _tenInts).
		Str("string", _tenStrings[0]).
		Interface("strings", _tenStrings).
		Time("time", _tenTimes[0]).
		Interface("times", _tenTimes).
		Interface("user1", _oneUser).
		Interface("user2", _oneUser).
		Interface("users", _tenUsers).
		Err(errExample)
}
