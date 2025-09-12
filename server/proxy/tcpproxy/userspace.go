// Copyright 2016 The etcd Authors
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

package tcpproxy

import (
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

type remote struct {
	srv      *net.SRV
	addr     string
	inactive atomic.Bool
}

func (r *remote) inactivate() {
	r.inactive.Store(true)
}

func (r *remote) tryReactivate() error {
	conn, err := net.Dial("tcp", r.addr)
	if err != nil {
		return err
	}
	conn.Close()
	r.inactive.Store(false)
	return nil
}

func (r *remote) isActive() bool {
	return !r.inactive.Load()
}

type TCPProxy struct {
	Logger          *zap.Logger
	Listener        net.Listener
	Endpoints       []*net.SRV
	MonitorInterval time.Duration
	DialTimeout     time.Duration

	mu        sync.Mutex // guards the following fields
	donec     chan struct{}
	remotes   []*remote
	pickCount uint64 // for round robin
}

func (tp *TCPProxy) Run() error {
	tp.mu.Lock()
	tp.donec = make(chan struct{})
	tp.mu.Unlock()

	if tp.MonitorInterval == 0 {
		tp.MonitorInterval = 5 * time.Minute
	}
	if tp.Logger == nil {
		tp.Logger = zap.NewNop()
	}
	if tp.DialTimeout == 0 {
		tp.DialTimeout = 10 * time.Second
	}

	eps := make([]string, 0, len(tp.Endpoints)) // for logging
	for _, srv := range tp.Endpoints {
		addr := net.JoinHostPort(srv.Target, strconv.Itoa(int(srv.Port)))
		tp.remotes = append(tp.remotes, &remote{srv: srv, addr: addr})
		eps = append(eps, addr)
	}
	tp.Logger.Info("ready to proxy client requests", zap.Strings("endpoints", eps))

	go tp.runMonitor()
	for {
		in, err := tp.Listener.Accept()
		if err != nil {
			return err
		}

		go tp.serve(in)
	}
}

func (tp *TCPProxy) pick() *remote {
	var weighted []*remote
	var unweighted []*remote

	bestPr := uint16(65535)
	w := 0
	// find best priority class
	for _, r := range tp.remotes {
		switch {
		case !r.isActive():
		case r.srv.Priority < bestPr:
			bestPr = r.srv.Priority
			w = 0
			weighted = nil
			unweighted = nil
			fallthrough
		case r.srv.Priority == bestPr:
			if r.srv.Weight > 0 {
				weighted = append(weighted, r)
				w += int(r.srv.Weight)
			} else {
				unweighted = append(unweighted, r)
			}
		}
	}
	if weighted != nil {
		if len(unweighted) > 0 && rand.Intn(100) == 1 {
			// In the presence of records containing weights greater
			// than 0, records with weight 0 should have a very small
			// chance of being selected.
			r := unweighted[tp.pickCount%uint64(len(unweighted))]
			tp.pickCount++
			return r
		}
		// choose a uniform random number between 0 and the sum computed
		// (inclusive), and select the RR whose running sum value is the
		// first in the selected order
		choose := rand.Intn(w)
		for i := range weighted {
			choose -= int(weighted[i].srv.Weight)
			if choose <= 0 {
				return weighted[i]
			}
		}
	}
	if unweighted != nil {
		for range tp.remotes {
			picked := tp.remotes[tp.pickCount%uint64(len(tp.remotes))]
			tp.pickCount++
			if picked.isActive() {
				return picked
			}
		}
	}
	return nil
}

func (tp *TCPProxy) serve(in net.Conn) {
	defer in.Close()

	var (
		err error
		out net.Conn
	)

	for {
		tp.mu.Lock()
		remote := tp.pick()
		tp.mu.Unlock()
		if remote == nil {
			return
		}
		out, err = net.DialTimeout("tcp", remote.addr, tp.DialTimeout)
		if err == nil {
			break
		}
		remote.inactivate()
		tp.Logger.Warn("deactivated endpoint", zap.String("address", remote.addr), zap.Duration("interval", tp.MonitorInterval), zap.Error(err))
	}

	defer out.Close()
	go io.Copy(in, out)
	io.Copy(out, in)
}

func (tp *TCPProxy) runMonitor() {
	timer := time.NewTimer(tp.MonitorInterval)
	defer timer.Stop()
	for {
		timer.Reset(tp.MonitorInterval)
		select {
		case <-timer.C:
			tp.mu.Lock()
			for _, rem := range tp.remotes {
				if rem.isActive() {
					continue
				}
				go func(r *remote) {
					if err := r.tryReactivate(); err != nil {
						tp.Logger.Warn("failed to activate endpoint (stay inactive for another interval)", zap.String("address", r.addr), zap.Duration("interval", tp.MonitorInterval), zap.Error(err))
					} else {
						tp.Logger.Info("activated", zap.String("address", r.addr))
					}
				}(rem)
			}
			tp.mu.Unlock()
		case <-tp.donec:
			return
		}
	}
}

func (tp *TCPProxy) Stop() {
	// graceful shutdown?
	// shutdown current connections?
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if tp.donec == nil {
		return // Never run.
	}
	select {
	case <-tp.donec:
		return // Already stopped.
	default:
	}

	tp.Listener.Close()
	close(tp.donec)
}
