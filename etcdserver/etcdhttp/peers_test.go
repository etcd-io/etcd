package etcdhttp

import "testing"

//TODO: full testing for peer set
func TestPeerSet(t *testing.T) {
	p := &Peers{}
	tests := []string{
		"1=1.1.1.1",
		"2=2.2.2.2",
		"1=1.1.1.1&1=1.1.1.2&2=2.2.2.2",
	}
	for i, tt := range tests {
		p.Set(tt)
		if p.String() != tt {
			t.Errorf("#%d: string = %s, want %s", i, p.String(), tt)
		}
	}
}
