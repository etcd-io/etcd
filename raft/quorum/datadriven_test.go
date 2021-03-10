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

package quorum

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/datadriven"
)

// TestDataDriven parses and executes the test cases in ./testdata/*. An entry
// in such a file specifies the command, which is either of "committed" to check
// CommittedIndex or "vote" to verify a VoteResult. The underlying configuration
// and inputs are specified via the arguments 'cfg' and 'cfgj' (for the majority
// config and, optionally, majority config joint to the first one) and 'idx'
// (for CommittedIndex) and 'votes' (for VoteResult).
//
// Internally, the harness runs some additional checks on each test case for
// which it is known that the result shouldn't change. For example,
// interchanging the majority configurations of a joint quorum must not
// influence the result; if it does, this is noted in the test's output.
func TestDataDriven(t *testing.T) {
	datadriven.Walk(t, "testdata", func(t *testing.T, path string) {
		datadriven.RunTest(t, path, func(t *testing.T, d *datadriven.TestData) string {
			// Two majority configs. The first one is always used (though it may
			// be empty) and the second one is used iff joint is true.
			var joint bool
			var ids, idsj []uint64
			// The committed indexes for the nodes in the config in the order in
			// which they appear in (ids,idsj), without repetition. An underscore
			// denotes an omission (i.e. no information for this voter); this is
			// different from 0. For example,
			//
			// cfg=(1,2) cfgj=(2,3,4) idxs=(_,5,_,7) initializes the idx for voter 2
			// to 5 and that for voter 4 to 7 (and no others).
			//
			// cfgj=zero is specified to instruct the test harness to treat cfgj
			// as zero instead of not specified (i.e. it will trigger a joint
			// quorum test instead of a majority quorum test for cfg only).
			var idxs []Index
			// Votes. These are initialized similar to idxs except the only values
			// used are 1 (voted against) and 2 (voted for). This looks awkward,
			// but is convenient because it allows sharing code between the two.
			var votes []Index

			// Parse the args.
			for _, arg := range d.CmdArgs {
				for i := range arg.Vals {
					switch arg.Key {
					case "cfg":
						var n uint64
						arg.Scan(t, i, &n)
						ids = append(ids, n)
					case "cfgj":
						joint = true
						if arg.Vals[i] == "zero" {
							if len(arg.Vals) != 1 {
								t.Fatalf("cannot mix 'zero' into configuration")
							}
						} else {
							var n uint64
							arg.Scan(t, i, &n)
							idsj = append(idsj, n)
						}
					case "idx":
						var n uint64
						// Register placeholders as zeroes.
						if arg.Vals[i] != "_" {
							arg.Scan(t, i, &n)
							if n == 0 {
								// This is a restriction caused by the above
								// special-casing for _.
								t.Fatalf("cannot use 0 as idx")
							}
						}
						idxs = append(idxs, Index(n))
					case "votes":
						var s string
						arg.Scan(t, i, &s)
						switch s {
						case "y":
							votes = append(votes, 2)
						case "n":
							votes = append(votes, 1)
						case "_":
							votes = append(votes, 0)
						default:
							t.Fatalf("unknown vote: %s", s)
						}
					default:
						t.Fatalf("unknown arg %s", arg.Key)
					}
				}
			}

			// Build the two majority configs.
			c := MajorityConfig{}
			for _, id := range ids {
				c[id] = struct{}{}
			}
			cj := MajorityConfig{}
			for _, id := range idsj {
				cj[id] = struct{}{}
			}

			// Helper that returns an AckedIndexer which has the specified indexes
			// mapped to the right IDs.
			makeLookuper := func(idxs []Index, ids, idsj []uint64) mapAckIndexer {
				l := mapAckIndexer{}
				var p int // next to consume from idxs
				for _, id := range append(append([]uint64(nil), ids...), idsj...) {
					if _, ok := l[id]; ok {
						continue
					}
					if p < len(idxs) {
						// NB: this creates zero entries for placeholders that we remove later.
						// The upshot of doing it that way is to avoid having to specify place-
						// holders multiple times when omitting voters present in both halves of
						// a joint config.
						l[id] = idxs[p]
						p++
					}
				}

				for id := range l {
					// Zero entries are created by _ placeholders; we don't want
					// them in the lookuper because "no entry" is different from
					// "zero entry". Note that we prevent tests from specifying
					// zero commit indexes, so that there's no confusion between
					// the two concepts.
					if l[id] == 0 {
						delete(l, id)
					}
				}
				return l
			}

			{
				input := idxs
				if d.Cmd == "vote" {
					input = votes
				}
				if voters := JointConfig([2]MajorityConfig{c, cj}).IDs(); len(voters) != len(input) {
					return fmt.Sprintf("error: mismatched input (explicit or _) for voters %v: %v",
						voters, input)
				}
			}

			var buf strings.Builder
			switch d.Cmd {
			case "committed":
				l := makeLookuper(idxs, ids, idsj)

				// Branch based on whether this is a majority or joint quorum
				// test case.
				if !joint {
					idx := c.CommittedIndex(l)
					fmt.Fprint(&buf, c.Describe(l))
					// These alternative computations should return the same
					// result. If not, print to the output.
					if aIdx := alternativeMajorityCommittedIndex(c, l); aIdx != idx {
						fmt.Fprintf(&buf, "%s <-- via alternative computation\n", aIdx)
					}
					// Joining a majority with the empty majority should give same result.
					if aIdx := JointConfig([2]MajorityConfig{c, {}}).CommittedIndex(l); aIdx != idx {
						fmt.Fprintf(&buf, "%s <-- via zero-joint quorum\n", aIdx)
					}
					// Joining a majority with itself should give same result.
					if aIdx := JointConfig([2]MajorityConfig{c, c}).CommittedIndex(l); aIdx != idx {
						fmt.Fprintf(&buf, "%s <-- via self-joint quorum\n", aIdx)
					}
					overlay := func(c MajorityConfig, l AckedIndexer, id uint64, idx Index) AckedIndexer {
						ll := mapAckIndexer{}
						for iid := range c {
							if iid == id {
								ll[iid] = idx
							} else if idx, ok := l.AckedIndex(iid); ok {
								ll[iid] = idx
							}
						}
						return ll
					}
					for id := range c {
						iidx, _ := l.AckedIndex(id)
						if idx > iidx && iidx > 0 {
							// If the committed index was definitely above the currently
							// inspected idx, the result shouldn't change if we lower it
							// further.
							lo := overlay(c, l, id, iidx-1)
							if aIdx := c.CommittedIndex(lo); aIdx != idx {
								fmt.Fprintf(&buf, "%s <-- overlaying %d->%d", aIdx, id, iidx)
							}
							lo = overlay(c, l, id, 0)
							if aIdx := c.CommittedIndex(lo); aIdx != idx {
								fmt.Fprintf(&buf, "%s <-- overlaying %d->0", aIdx, id)
							}
						}
					}
					fmt.Fprintf(&buf, "%s\n", idx)
				} else {
					cc := JointConfig([2]MajorityConfig{c, cj})
					fmt.Fprint(&buf, cc.Describe(l))
					idx := cc.CommittedIndex(l)
					// Interchanging the majorities shouldn't make a difference. If it does, print.
					if aIdx := JointConfig([2]MajorityConfig{cj, c}).CommittedIndex(l); aIdx != idx {
						fmt.Fprintf(&buf, "%s <-- via symmetry\n", aIdx)
					}
					fmt.Fprintf(&buf, "%s\n", idx)
				}
			case "vote":
				ll := makeLookuper(votes, ids, idsj)
				l := map[uint64]bool{}
				for id, v := range ll {
					l[id] = v != 1 // NB: 1 == false, 2 == true
				}

				if !joint {
					// Test a majority quorum.
					r := c.VoteResult(l)
					fmt.Fprintf(&buf, "%v\n", r)
				} else {
					// Run a joint quorum test case.
					r := JointConfig([2]MajorityConfig{c, cj}).VoteResult(l)
					// Interchanging the majorities shouldn't make a difference. If it does, print.
					if ar := JointConfig([2]MajorityConfig{cj, c}).VoteResult(l); ar != r {
						fmt.Fprintf(&buf, "%v <-- via symmetry\n", ar)
					}
					fmt.Fprintf(&buf, "%v\n", r)
				}
			default:
				t.Fatalf("unknown command: %s", d.Cmd)
			}
			return buf.String()
		})
	})
}
