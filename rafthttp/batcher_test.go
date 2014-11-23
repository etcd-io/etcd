package rafthttp

import (
	"testing"
	"time"
)

func TestBatcherNum(t *testing.T) {
	n := 100
	largeD := time.Minute
	tests := []struct {
		n         int
		wnotbatch int
	}{
		{n - 1, 0},
		{n, 1},
		{n + 1, 1},
		{2*n + 1, 2},
		{3*n + 1, 3},
	}

	for i, tt := range tests {
		b := NewBatcher(n, largeD)
		notbatched := 0
		for j := 0; j < tt.n; j++ {
			if !b.ShouldBatch(time.Now()) {
				notbatched++
			}
		}
		if notbatched != tt.wnotbatch {
			t.Errorf("#%d: notbatched = %d, want %d", i, notbatched, tt.wnotbatch)
		}
	}
}

func TestBatcherTime(t *testing.T) {
	largeN := 10000
	tests := []struct {
		nms       int
		wnotbatch int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 3},
	}

	for i, tt := range tests {
		b := NewBatcher(largeN, time.Millisecond)
		baseT := b.batchedT
		notbatched := 0
		for j := 0; j < tt.nms+1; j++ {
			if !b.ShouldBatch(baseT.Add(time.Duration(j) * time.Millisecond)) {
				notbatched++
			}
		}
		if notbatched != tt.wnotbatch {
			t.Errorf("#%d: notbatched = %d, want %d", i, notbatched, tt.wnotbatch)
		}
	}
}
