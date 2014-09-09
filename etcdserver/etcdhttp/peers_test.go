package etcdhttp

import "testing"

//TODO: full testing for peer set
func TestPeerSet(t *testing.T) {
	p := &Peers{}

	p.Set("0x1=1.1.1.1")
	if p.Pick(0x1) != "http://1.1.1.1" {
		t.Errorf("pick = %s, want http://1.1.1.1", p.Pick(0x1))
	}

	p.Set("0x2=2.2.2.2")
	if p.Pick(0x1) != "" {
		t.Errorf("pick = %s, want empty string", p.Pick(0x1))
	}
	if p.Pick(0x2) != "http://2.2.2.2" {
		t.Errorf("pick = %s, want http://2.2.2.2", p.Pick(0x2))
	}
}
