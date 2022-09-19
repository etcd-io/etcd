// Copyright 2022 The etcd Authors
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

package raft

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type nodeTestHarness struct {
	*node
	t *testing.T
}

func (l *nodeTestHarness) Debug(v ...interface{}) {
	l.t.Log(v...)
}

func (l *nodeTestHarness) Debugf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

func (l *nodeTestHarness) Error(v ...interface{}) {
	l.t.Error(v...)
}

func (l *nodeTestHarness) Errorf(format string, v ...interface{}) {
	l.t.Errorf(format, v...)
}

func (l *nodeTestHarness) Info(v ...interface{}) {
	l.t.Log(v...)
}

func (l *nodeTestHarness) Infof(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

func (l *nodeTestHarness) Warning(v ...interface{}) {
	l.t.Log(v...)
}

func (l *nodeTestHarness) Warningf(format string, v ...interface{}) {
	l.t.Logf(format, v...)
}

func (l *nodeTestHarness) Fatal(v ...interface{}) {
	l.t.Error(v...)
	panic(v)
}

func (l *nodeTestHarness) Fatalf(format string, v ...interface{}) {
	l.t.Errorf(format, v...)
	panic(fmt.Sprintf(format, v...))
}

func (l *nodeTestHarness) Panic(v ...interface{}) {
	l.t.Log(v...)
	panic(v)
}

func (l *nodeTestHarness) Panicf(format string, v ...interface{}) {
	l.t.Errorf(format, v...)
	panic(fmt.Sprintf(format, v...))
}

func newNodeTestHarness(ctx context.Context, t *testing.T, cfg *Config, peers ...Peer) (_ context.Context, cancel func(), _ *nodeTestHarness) {
	// Wrap context in a 10s timeout to make tests more robust. Otherwise,
	// it's likely that deadlock will occur unless Node behaves exactly as
	// expected - when you expect a Ready and start waiting on the channel
	// but no Ready ever shows up, for example.
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	var n *node
	if len(peers) > 0 {
		n = setupNode(cfg, peers)
	} else {
		rn, err := NewRawNode(cfg)
		if err != nil {
			t.Fatal(err)
		}
		nn := newNode(rn)
		n = &nn
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Error(r)
			}
		}()
		defer cancel()
		defer n.Stop()
		n.run()
	}()
	t.Cleanup(n.Stop)
	return ctx, cancel, &nodeTestHarness{node: n, t: t}
}
