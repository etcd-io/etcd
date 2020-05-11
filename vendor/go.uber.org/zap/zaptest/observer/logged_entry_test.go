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

package observer

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
)

func TestLoggedEntryContextMap(t *testing.T) {
	tests := []struct {
		msg    string
		fields []zapcore.Field
		want   map[string]interface{}
	}{
		{
			msg:    "no fields",
			fields: nil,
			want:   map[string]interface{}{},
		},
		{
			msg: "simple",
			fields: []zapcore.Field{
				zap.String("k1", "v"),
				zap.Int64("k2", 10),
			},
			want: map[string]interface{}{
				"k1": "v",
				"k2": int64(10),
			},
		},
		{
			msg: "overwrite",
			fields: []zapcore.Field{
				zap.String("k1", "v1"),
				zap.String("k1", "v2"),
			},
			want: map[string]interface{}{
				"k1": "v2",
			},
		},
		{
			msg: "nested",
			fields: []zapcore.Field{
				zap.String("k1", "v1"),
				zap.Namespace("nested"),
				zap.String("k2", "v2"),
			},
			want: map[string]interface{}{
				"k1": "v1",
				"nested": map[string]interface{}{
					"k2": "v2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			entry := LoggedEntry{
				Context: tt.fields,
			}
			assert.Equal(t, tt.want, entry.ContextMap())
		})
	}
}
