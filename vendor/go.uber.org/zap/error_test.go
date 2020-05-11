// Copyright (c) 2017 Uber Technologies, Inc.
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

package zap

import (
	"errors"
	"testing"

	"go.uber.org/zap/zapcore"

	richErrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorConstructors(t *testing.T) {
	fail := errors.New("fail")

	tests := []struct {
		name   string
		field  Field
		expect Field
	}{
		{"Error", Skip(), Error(nil)},
		{"Error", Field{Key: "error", Type: zapcore.ErrorType, Interface: fail}, Error(fail)},
		{"NamedError", Skip(), NamedError("foo", nil)},
		{"NamedError", Field{Key: "foo", Type: zapcore.ErrorType, Interface: fail}, NamedError("foo", fail)},
		{"Any:Error", Any("k", errors.New("v")), NamedError("k", errors.New("v"))},
		{"Any:Errors", Any("k", []error{errors.New("v")}), Errors("k", []error{errors.New("v")})},
	}

	for _, tt := range tests {
		if !assert.Equal(t, tt.expect, tt.field, "Unexpected output from convenience field constructor %s.", tt.name) {
			t.Logf("type expected: %T\nGot: %T", tt.expect.Interface, tt.field.Interface)
		}
		assertCanBeReused(t, tt.field)
	}
}

func TestErrorArrayConstructor(t *testing.T) {
	tests := []struct {
		desc     string
		field    Field
		expected []interface{}
	}{
		{"empty errors", Errors("", []error{}), []interface{}{}},
		{
			"errors",
			Errors("", []error{nil, errors.New("foo"), nil, errors.New("bar")}),
			[]interface{}{map[string]interface{}{"error": "foo"}, map[string]interface{}{"error": "bar"}},
		},
	}

	for _, tt := range tests {
		enc := zapcore.NewMapObjectEncoder()
		tt.field.Key = "k"
		tt.field.AddTo(enc)
		assert.Equal(t, tt.expected, enc.Fields["k"], "%s: unexpected map contents.", tt.desc)
		assert.Equal(t, 1, len(enc.Fields), "%s: found extra keys in map: %v", tt.desc, enc.Fields)
	}
}

func TestErrorsArraysHandleRichErrors(t *testing.T) {
	errs := []error{richErrors.New("egad")}

	enc := zapcore.NewMapObjectEncoder()
	Errors("k", errs).AddTo(enc)
	assert.Equal(t, 1, len(enc.Fields), "Expected only top-level field.")

	val := enc.Fields["k"]
	arr, ok := val.([]interface{})
	require.True(t, ok, "Expected top-level field to be an array.")
	require.Equal(t, 1, len(arr), "Expected only one error object in array.")

	serialized := arr[0]
	errMap, ok := serialized.(map[string]interface{})
	require.True(t, ok, "Expected serialized error to be a map, got %T.", serialized)
	assert.Equal(t, "egad", errMap["error"], "Unexpected standard error string.")
	assert.Contains(t, errMap["errorVerbose"], "egad", "Verbose error string should be a superset of standard error.")
	assert.Contains(t, errMap["errorVerbose"], "TestErrorsArraysHandleRichErrors", "Verbose error string should contain a stacktrace.")
}
