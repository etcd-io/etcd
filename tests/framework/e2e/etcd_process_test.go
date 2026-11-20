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

package e2e

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"go.etcd.io/etcd/pkg/v3/expect"
)

func newWaitingProcess(t *testing.T) *expect.ExpectProcess {
	t.Helper()
	proc, err := expect.NewExpect("/bin/sh", "-c", "echo ready; read line; exit 7")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = proc.Stop()
		_ = proc.Close()
	})
	_, err = proc.ExpectWithContext(t.Context(), expect.ExpectedResponse{Value: "ready"})
	require.NoError(t, err)
	return proc
}

func TestEtcdServerProcessCanceledWaitKeepsReplacement(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	core, logs := observer.New(zap.InfoLevel)
	old := newWaitingProcess(t)
	ep := &EtcdServerProcess{cfg: &EtcdServerProcessConfig{lg: zap.New(core)}, proc: old}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, ep.Wait(ctx), context.Canceled)

	// The canceled wait must retain the old process, even if its goroutine
	// has not run before a replacement is installed.
	replacement := newWaitingProcess(t)
	ep.procMu.Lock()
	ep.proc = replacement
	ep.procMu.Unlock()
	require.NoError(t, old.Send("done\n"))
	require.Eventually(t, func() bool {
		return logs.FilterMessage("server exited").Len() == 1
	}, 5*time.Second, time.Millisecond)
	require.NotContains(t, logs.FilterMessage("server exited").All()[0].ContextMap(), "error")
	require.EqualValues(t, 7, logs.FilterMessage("server exited").All()[0].ContextMap()["code"])
	require.Same(t, replacement, ep.getProcess())
	ep.clearProcess(old)
	require.Same(t, replacement, ep.getProcess())
}

func TestEtcdServerProcessCanceledWaitSurvivesClear(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	proc := newWaitingProcess(t)
	ep := &EtcdServerProcess{cfg: &EtcdServerProcessConfig{lg: zap.New(core)}, proc: proc}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, ep.Wait(ctx), context.Canceled)
	ep.clearProcess(proc)
	require.NoError(t, proc.Send("done\n"))
	require.Eventually(t, func() bool {
		return logs.FilterMessage("server exited").Len() == 1
	}, 5*time.Second, time.Millisecond)
	require.Nil(t, ep.getProcess())
}

func TestEtcdServerProcessWaitConcurrentStop(t *testing.T) {
	proc := newWaitingProcess(t)
	donec := make(chan struct{})
	close(donec)
	ep := &EtcdServerProcess{cfg: &EtcdServerProcessConfig{lg: zap.NewNop()}, proc: proc, donec: donec}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, ep.Wait(ctx), context.Canceled)
	observed := make(chan struct{})
	go func() {
		defer close(observed)
		for ep.IsRunning() {
			runtime.Gosched()
		}
	}()
	require.NoError(t, ep.Stop())
	select {
	case <-observed:
	case <-time.After(5 * time.Second):
		t.Fatal("process observer did not finish after stop")
	}
	require.Nil(t, ep.getProcess())
}
