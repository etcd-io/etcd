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

package zap

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap/internal/ztest"
	"go.uber.org/zap/zapcore"
)

type user struct {
	Name      string
	Email     string
	CreatedAt time.Time
}

func (u *user) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", u.Name)
	enc.AddString("email", u.Email)
	enc.AddInt64("created_at", u.CreatedAt.UnixNano())
	return nil
}

var _jane = &user{
	Name:      "Jane Doe",
	Email:     "jane@test.com",
	CreatedAt: time.Date(1980, 1, 1, 12, 0, 0, 0, time.UTC),
}

func withBenchedLogger(b *testing.B, f func(*Logger)) {
	logger := New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(NewProductionConfig().EncoderConfig),
			&ztest.Discarder{},
			DebugLevel,
		))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			f(logger)
		}
	})
}

func BenchmarkNoContext(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("No context.")
	})
}

func BenchmarkBoolField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Boolean.", Bool("foo", true))
	})
}

func BenchmarkByteStringField(b *testing.B) {
	val := []byte("bar")
	withBenchedLogger(b, func(log *Logger) {
		log.Info("ByteString.", ByteString("foo", val))
	})
}

func BenchmarkFloat64Field(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Floating point.", Float64("foo", 3.14))
	})
}

func BenchmarkIntField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Integer.", Int("foo", 42))
	})
}

func BenchmarkInt64Field(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("64-bit integer.", Int64("foo", 42))
	})
}

func BenchmarkStringField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Strings.", String("foo", "bar"))
	})
}

func BenchmarkStringerField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Level.", Stringer("foo", InfoLevel))
	})
}

func BenchmarkTimeField(b *testing.B) {
	t := time.Unix(0, 0)
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Time.", Time("foo", t))
	})
}

func BenchmarkDurationField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Duration", Duration("foo", time.Second))
	})
}

func BenchmarkErrorField(b *testing.B) {
	err := errors.New("egad")
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Error.", Error(err))
	})
}

func BenchmarkErrorsField(b *testing.B) {
	errs := []error{
		errors.New("egad"),
		errors.New("oh no"),
		errors.New("dear me"),
		errors.New("such fail"),
	}
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Errors.", Errors("errors", errs))
	})
}

func BenchmarkStackField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Error.", Stack("stacktrace"))
	})
}

func BenchmarkObjectField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Arbitrary ObjectMarshaler.", Object("user", _jane))
	})
}

func BenchmarkReflectField(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Reflection-based serialization.", Reflect("user", _jane))
	})
}

func BenchmarkAddCallerHook(b *testing.B) {
	logger := New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(NewProductionConfig().EncoderConfig),
			&ztest.Discarder{},
			InfoLevel,
		),
		AddCaller(),
	)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("Caller.")
		}
	})
}

func Benchmark10Fields(b *testing.B) {
	withBenchedLogger(b, func(log *Logger) {
		log.Info("Ten fields, passed at the log site.",
			Int("one", 1),
			Int("two", 2),
			Int("three", 3),
			Int("four", 4),
			Int("five", 5),
			Int("six", 6),
			Int("seven", 7),
			Int("eight", 8),
			Int("nine", 9),
			Int("ten", 10),
		)
	})
}

func Benchmark100Fields(b *testing.B) {
	const batchSize = 50
	logger := New(zapcore.NewCore(
		zapcore.NewJSONEncoder(NewProductionConfig().EncoderConfig),
		&ztest.Discarder{},
		DebugLevel,
	))

	// Don't include allocating these helper slices in the benchmark. Since
	// access to them isn't synchronized, we can't run the benchmark in
	// parallel.
	first := make([]Field, batchSize)
	second := make([]Field, batchSize)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for i := 0; i < batchSize; i++ {
			// We're duplicating keys, but that doesn't affect performance.
			first[i] = Int("foo", i)
			second[i] = Int("foo", i+batchSize)
		}
		logger.With(first...).Info("Child loggers with lots of context.", second...)
	}
}
