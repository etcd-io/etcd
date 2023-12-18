// Copyright 2017 The etcd Authors
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
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/pkg/expect"
	"go.etcd.io/etcd/pkg/testutil"
)

func waitReadyExpectProc(exproc *expect.ExpectProcess, readyStrs []string) error {
	c := 0
	matchSet := func(l string) bool {
		for _, s := range readyStrs {
			if strings.Contains(l, s) {
				c++
				break
			}
		}
		return c == len(readyStrs)
	}
	_, err := exproc.ExpectFunc(matchSet)
	return err
}

func spawnWithExpect(args []string, expected string) error {
	return spawnWithExpects(args, []string{expected}...)
}

func spawnWithExpects(args []string, xs ...string) error {
	_, err := spawnWithExpectLines(args, xs...)
	return err
}

func spawnWithExpectLines(args []string, xs ...string) ([]string, error) {
	proc, err := spawnCmd(args)
	if err != nil {
		return nil, err
	}
	// process until either stdout or stderr contains
	// the expected string
	var (
		lines    []string
		lineFunc = func(txt string) bool { return true }
	)
	for _, txt := range xs {
		for {
			l, lerr := proc.ExpectFunc(lineFunc)
			if lerr != nil {
				proc.Close()
				return nil, fmt.Errorf("%v (expected %q, got %q)", lerr, txt, lines)
			}
			lines = append(lines, l)
			if strings.Contains(l, txt) {
				break
			}
		}
	}
	perr := proc.Close()
	if len(xs) == 0 && proc.LineCount() != noOutputLineCount { // expect no output
		return nil, fmt.Errorf("unexpected output (got lines %q, line count %d)", lines, proc.LineCount())
	}
	return lines, perr
}

func randomLeaseID() int64 {
	return rand.New(rand.NewSource(time.Now().UnixNano())).Int63()
}

func dataMarshal(data interface{}) (d string, e error) {
	m, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(m), nil
}

func closeWithTimeout(p *expect.ExpectProcess, d time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- p.Close() }()
	select {
	case err := <-errc:
		return err
	case <-time.After(d):
		p.Stop()
		// retry close after stopping to collect SIGQUIT data, if any
		closeWithTimeout(p, time.Second)
	}
	return fmt.Errorf("took longer than %v to Close process %+v", d, p)
}

func toTLS(s string) string {
	return strings.Replace(s, "http://", "https://", 1)
}

func executeUntil(ctx context.Context, t *testing.T, f func()) {
	deadline, deadlineSet := ctx.Deadline()
	timeout := time.Until(deadline)
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		f()
	}()

	select {
	case <-ctx.Done():
		msg := ctx.Err().Error()
		if deadlineSet {
			msg = fmt.Sprintf("test timed out after %v, err: %v", timeout, msg)
		}
		testutil.FatalStack(t, msg)
	case <-donec:
	}
}

func corruptBBolt(fpath string) error {
	db, derr := bbolt.Open(fpath, os.ModePerm, &bbolt.Options{})
	if derr != nil {
		return derr
	}
	defer db.Close()

	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("key"))
		if b == nil {
			return errors.New("got nil bucket for 'key'")
		}
		keys, vals := [][]byte{}, [][]byte{}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			keys = append(keys, k)
			var kv mvccpb.KeyValue
			if uerr := kv.Unmarshal(v); uerr != nil {
				return uerr
			}
			kv.Key[0]++
			kv.Value[0]++
			v2, v2err := kv.Marshal()
			if v2err != nil {
				return v2err
			}
			vals = append(vals, v2)
		}
		for i := range keys {
			if perr := b.Put(keys[i], vals[i]); perr != nil {
				return perr
			}
		}
		return nil
	})
}
