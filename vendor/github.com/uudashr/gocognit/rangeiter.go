package gocognit

import (
	"fmt"
	"iter"
)

func ForRangeIter(all iter.Seq[int]) {
	for v := range all {
		fmt.Println(v)
	}
}

func CountDown(start int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := start; i > 0; i-- {
			if !yield(i) {
				return
			}
		}
	}
}

func DoCount() {
	for n := range CountDown(5) {
		fmt.Println(n)
	}
}

func FilteredMap(m map[string]int, threshold int) iter.Seq2[string, int] {
	return func(yield func(string, int) bool) {
		for k, v := range m {
			if v > threshold {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}
func DoFilter() {
	scores := map[string]int{"Alice": 50, "Bob": 90, "Charlie": 85}

	for name, score := range FilteredMap(scores, 80) {
		fmt.Printf("%s: %d\n", name, score)
	}
}
