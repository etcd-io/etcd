package etcdhttp

import (
	"reflect"
	"sort"
	"testing"
)

func TestCluster(t *testing.T) {
	tests := []struct {
		in      string
		wids    []int64
		wep     Endpoints
		wstring string
	}{
		{
			"mach1=1.1.1.1",
			[]int64{6766124441145243813},
			[]string{"http://1.1.1.1"},
			"5de61a54ac8cf8a5=1.1.1.1",
		},
		{
			"mach2=2.2.2.2",
			[]int64{4216210504111742903},
			[]string{"http://2.2.2.2"},
			"3a82fd1d73d9dfb7=2.2.2.2",
		},
		{
			"mach1=1.1.1.1,1.1.1.2 mach2=2.2.2.2",
			[]int64{4216210504111742903, 6766124441145243813},
			[]string{"http://1.1.1.1", "http://1.1.1.2", "http://2.2.2.2"},
			"3a82fd1d73d9dfb7=2.2.2.2&5de61a54ac8cf8a5=1.1.1.1&5de61a54ac8cf8a5=1.1.1.2",
		},
		{
			"mach3=3.3.3.3 mach4=4.4.4.4 mach1=1.1.1.1,1.1.1.2 mach2=2.2.2.2",
			[]int64{539760383923670193, 1066726282636464211, 4216210504111742903, 6766124441145243813},
			[]string{"http://1.1.1.1", "http://1.1.1.2", "http://2.2.2.2",
				"http://3.3.3.3", "http://4.4.4.4"},
			"3a82fd1d73d9dfb7=2.2.2.2&5de61a54ac8cf8a5=1.1.1.1&5de61a54ac8cf8a5=1.1.1.2&77d9d359b994cb1=3.3.3.3&ecdc5e6fd1e9053=4.4.4.4",
		},
	}
	for i, tt := range tests {
		p := &Cluster{}
		err := p.Set(tt.in)
		if err != nil {
			t.Errorf("#%d: err=%v, want nil", i, err)
		}
		ids := int64Slice(p.IDs())
		sort.Sort(ids)
		if !reflect.DeepEqual([]int64(ids), tt.wids) {
			t.Errorf("#%d: IDs=%#v, want %#v", i, []int64(ids), tt.wids)
		}
		ep := p.Endpoints()
		if !reflect.DeepEqual(ep, tt.wep) {
			t.Errorf("#%d: Endpoints=%#v, want %#v", i, ep, tt.wep)
		}
		s := p.String()
		if s != tt.wstring {
			t.Errorf("#%d: string=%q, want %q", i, s, tt.wstring)
		}
	}
}

type int64Slice []int64

func (p int64Slice) Len() int           { return len(p) }
func (p int64Slice) Less(i, j int) bool { return p[i] < p[j] }
func (p int64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func TestClusterSetBad(t *testing.T) {
	tests := []string{
		// garbage URL
		"asdf%%",
	}
	for i, tt := range tests {
		p := &Cluster{}
		if err := p.Set(tt); err == nil {
			t.Errorf("#%d: err=nil unexpectedly", i)
		}
	}
}

func TestClusterPick(t *testing.T) {
	ps := &Cluster{
		1: []string{"abc", "def", "ghi", "jkl", "mno", "pqr", "stu"},
		2: []string{"xyz"},
		3: []string{},
	}
	ids := map[string]bool{
		"http://abc": true,
		"http://def": true,
		"http://ghi": true,
		"http://jkl": true,
		"http://mno": true,
		"http://pqr": true,
		"http://stu": true,
	}
	for i := 0; i < 1000; i++ {
		a := ps.Pick(1)
		if _, ok := ids[a]; !ok {
			t.Errorf("returned ID %q not in expected range!", a)
			break
		}
	}
	if b := ps.Pick(2); b != "http://xyz" {
		t.Errorf("id=%q, want %q", b, "http://xyz")
	}
	if c := ps.Pick(3); c != "" {
		t.Errorf("id=%q, want \"\"", c)
	}
}

func TestNodeName(t *testing.T) {
	tests := []struct {
		in string
		id int64
	}{
		{
			"mach1",
			0x5de61a54ac8cf8a5,
		},
	}

	for i, tt := range tests {
		var n NodeName
		err := n.Set(tt.in)
		if err != nil {
			t.Errorf("#%d: err=%v, want nil", i, err)
		}
		if n.ID() != tt.id {
			t.Errorf("#%d: err=%v, want nil", i, err)
		}
	}

}
