package merge

import (
	"testing"

	"github.com/gogo/protobuf/proto"
)

func TestClone1(t *testing.T) {
	f1 := &A{}
	proto.Clone(f1)
}

func TestMerge1(t *testing.T) {
	f1 := &A{}
	f2 := &A{}
	proto.Merge(f1, f2)
}

func TestMerge2(t *testing.T) {
	f1 := &A{B: B{C: 1}}
	f2 := &A{}
	proto.Merge(f1, f2)
	if f1.B.C != 1 {
		t.Fatalf("got %d want %d", f1.B.C, 1)
	}
}

func TestMerge3(t *testing.T) {
	f1 := &A{}
	f2 := &A{B: B{C: 1}}
	proto.Merge(f1, f2)
	if f1.B.C != 1 {
		t.Fatalf("got %d want %d", f1.B.C, 1)
	}
}
