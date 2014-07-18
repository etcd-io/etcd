package etcd

import (
	"math/rand"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/etcd/config"
)

func TestKillLeader(t *testing.T) {
	tests := []int{3, 5, 9, 11}

	for i, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)
		waitLeader(es)

		lead := es[0].p.node.Leader()
		es[lead].Stop()

		time.Sleep(es[0].tickDuration * defaultElection * 2)

		waitLeader(es)
		if es[1].p.node.Leader() == 0 {
			t.Errorf("#%d: lead = %d, want not 0", i, es[1].p.node.Leader())
		}

		for i := range es {
			es[len(es)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

func TestRandomKill(t *testing.T) {
	tests := []int{3, 5, 9, 11}

	for _, tt := range tests {
		es, hs := buildCluster(tt, false)
		waitCluster(t, es)
		waitLeader(es)

		toKill := make(map[int64]struct{})
		for len(toKill) != tt/2-1 {
			toKill[rand.Int63n(int64(tt))] = struct{}{}
		}
		for k := range toKill {
			es[k].Stop()
		}

		time.Sleep(es[0].tickDuration * defaultElection * 2)

		waitLeader(es)

		for i := range es {
			es[len(es)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

func TestJoinThroughFollower(t *testing.T) {
	tests := []int{3, 4, 5, 6}

	for _, tt := range tests {
		es := make([]*Server, tt)
		hs := make([]*httptest.Server, tt)
		for i := 0; i < tt; i++ {
			c := config.New()
			if i > 0 {
				c.Peers = []string{hs[i-1].URL}
			}
			es[i], hs[i] = initTestServer(c, int64(i), false)
		}

		go es[0].Run()

		for i := 1; i < tt; i++ {
			go es[i].Run()
			waitLeader(es[:i])
		}
		waitCluster(t, es)

		for i := range hs {
			es[len(hs)-i-1].Stop()
		}
		for i := range hs {
			hs[len(hs)-i-1].Close()
		}
	}
	afterTest(t)
}

type leadterm struct {
	lead int64
	term int64
}

func waitActiveLeader(es []*Server) (lead, term int64) {
	for {
		if l, t := waitLeader(es); l >= 0 && es[l].mode.Get() == participantMode {
			return l, t
		}
	}
}

// waitLeader waits until all alive servers are checked to have the same leader.
// WARNING: The lead returned is not guaranteed to be actual leader.
func waitLeader(es []*Server) (lead, term int64) {
	for {
		ls := make([]leadterm, 0, len(es))
		for i := range es {
			switch es[i].mode.Get() {
			case participantMode:
				ls = append(ls, getLead(es[i]))
			case standbyMode:
				//TODO(xiangli) add standby support
			case stopMode:
			}
		}
		if isSameLead(ls) {
			return ls[0].lead, ls[0].term
		}
		time.Sleep(es[0].tickDuration * defaultElection)
	}
}

func getLead(s *Server) leadterm {
	return leadterm{s.p.node.Leader(), s.p.node.Term()}
}

func isSameLead(ls []leadterm) bool {
	m := make(map[leadterm]int)
	for i := range ls {
		m[ls[i]] = m[ls[i]] + 1
	}
	if len(m) == 1 {
		if ls[0].lead == -1 {
			return false
		}
		return true
	}
	// todo(xiangli): printout the current cluster status for debugging....
	return false
}
