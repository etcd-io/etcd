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

package validate

import (
	"time"

	"go.uber.org/zap"
)

type TimerLogger interface {
	Run(string, func(TimerLogger))
	Logger() *zap.Logger
}

func NewTimerLogger(lg *zap.Logger) TimerLogger {
	return &flatTimerLogger{lg}
}

type flatTimerLogger struct {
	lg *zap.Logger
}

func (t *flatTimerLogger) Run(name string, run func(t TimerLogger)) {
	t.lg.Info("Start " + name)
	start := time.Now()
	run(t)
	t.lg.Info("End "+name, zap.Duration("duration", time.Since(start)))
}

func (t *flatTimerLogger) Logger() *zap.Logger {
	return t.lg
}
