// Copyright 2019 The etcd Authors
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

package confchange

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	pb "go.etcd.io/etcd/raft/raftpb"
	"go.etcd.io/etcd/raft/tracker"
)

// TestConfChangeQuick uses quickcheck to verify that simple and joint config
// changes arrive at the same result.
func TestConfChangeQuick(t *testing.T) {
	cfg := &quick.Config{
		MaxCount: 1000,
	}

	// Log the first couple of runs to give some indication of things working
	// as intended.
	const infoCount = 5

	runWithJoint := func(c *Changer, ccs []pb.ConfChangeSingle) error {
		cfg, prs, err := c.EnterJoint(false /* autoLeave */, ccs...)
		if err != nil {
			return err
		}
		// Also do this with autoLeave on, just to check that we'd get the same
		// result.
		cfg2a, prs2a, err := c.EnterJoint(true /* autoLeave */, ccs...)
		if err != nil {
			return err
		}
		cfg2a.AutoLeave = false
		if !reflect.DeepEqual(cfg, cfg2a) || !reflect.DeepEqual(prs, prs2a) {
			return fmt.Errorf("cfg: %+v\ncfg2a: %+v\nprs: %+v\nprs2a: %+v",
				cfg, cfg2a, prs, prs2a)
		}
		c.Tracker.Config = cfg
		c.Tracker.Progress = prs
		cfg2b, prs2b, err := c.LeaveJoint()
		if err != nil {
			return err
		}
		// Reset back to the main branch with autoLeave=false.
		c.Tracker.Config = cfg
		c.Tracker.Progress = prs
		cfg, prs, err = c.LeaveJoint()
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(cfg, cfg2b) || !reflect.DeepEqual(prs, prs2b) {
			return fmt.Errorf("cfg: %+v\ncfg2b: %+v\nprs: %+v\nprs2b: %+v",
				cfg, cfg2b, prs, prs2b)
		}
		c.Tracker.Config = cfg
		c.Tracker.Progress = prs
		return nil
	}

	runWithSimple := func(c *Changer, ccs []pb.ConfChangeSingle) error {
		for _, cc := range ccs {
			cfg, prs, err := c.Simple(cc)
			if err != nil {
				return err
			}
			c.Tracker.Config, c.Tracker.Progress = cfg, prs
		}
		return nil
	}

	type testFunc func(*Changer, []pb.ConfChangeSingle) error

	wrapper := func(invoke testFunc) func(setup initialChanges, ccs confChanges) (*Changer, error) {
		return func(setup initialChanges, ccs confChanges) (*Changer, error) {
			tr := tracker.MakeProgressTracker(10)
			c := &Changer{
				Tracker:   tr,
				LastIndex: 10,
			}

			if err := runWithSimple(c, setup); err != nil {
				return nil, err
			}

			err := invoke(c, ccs)
			return c, err
		}
	}

	var n int
	f1 := func(setup initialChanges, ccs confChanges) *Changer {
		c, err := wrapper(runWithSimple)(setup, ccs)
		if err != nil {
			t.Fatal(err)
		}
		if n < infoCount {
			t.Log("initial setup:", Describe(setup...))
			t.Log("changes:", Describe(ccs...))
			t.Log(c.Tracker.Config)
			t.Log(c.Tracker.Progress)
		}
		n++
		return c
	}
	f2 := func(setup initialChanges, ccs confChanges) *Changer {
		c, err := wrapper(runWithJoint)(setup, ccs)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	err := quick.CheckEqual(f1, f2, cfg)
	if err == nil {
		return
	}
	cErr, ok := err.(*quick.CheckEqualError)
	if !ok {
		t.Fatal(err)
	}

	t.Error("setup:", Describe(cErr.In[0].([]pb.ConfChangeSingle)...))
	t.Error("ccs:", Describe(cErr.In[1].([]pb.ConfChangeSingle)...))
	t.Errorf("out1: %+v\nout2: %+v", cErr.Out1, cErr.Out2)
}

type confChangeTyp pb.ConfChangeType

func (confChangeTyp) Generate(rand *rand.Rand, _ int) reflect.Value {
	return reflect.ValueOf(confChangeTyp(rand.Intn(4)))
}

type confChanges []pb.ConfChangeSingle

func genCC(num func() int, id func() uint64, typ func() pb.ConfChangeType) []pb.ConfChangeSingle {
	var ccs []pb.ConfChangeSingle
	n := num()
	for i := 0; i < n; i++ {
		ccs = append(ccs, pb.ConfChangeSingle{Type: typ(), NodeID: id()})
	}
	return ccs
}

func (confChanges) Generate(rand *rand.Rand, _ int) reflect.Value {
	num := func() int {
		return 1 + rand.Intn(9)
	}
	id := func() uint64 {
		// Note that num() >= 1, so we're never returning 1 from this method,
		// meaning that we'll never touch NodeID one, which is special to avoid
		// voterless configs altogether in this test.
		return 1 + uint64(num())
	}
	typ := func() pb.ConfChangeType {
		return pb.ConfChangeType(rand.Intn(len(pb.ConfChangeType_name)))
	}
	return reflect.ValueOf(genCC(num, id, typ))
}

type initialChanges []pb.ConfChangeSingle

func (initialChanges) Generate(rand *rand.Rand, _ int) reflect.Value {
	num := func() int {
		return 1 + rand.Intn(5)
	}
	id := func() uint64 { return uint64(num()) }
	typ := func() pb.ConfChangeType {
		return pb.ConfChangeAddNode
	}
	// NodeID one is special - it's in the initial config and will be a voter
	// always (this is to avoid uninteresting edge cases where the simple conf
	// changes can't easily make progress).
	ccs := append([]pb.ConfChangeSingle{{Type: pb.ConfChangeAddNode, NodeID: 1}}, genCC(num, id, typ)...)
	return reflect.ValueOf(ccs)
}
