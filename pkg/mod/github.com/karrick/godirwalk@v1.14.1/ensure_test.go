package godirwalk

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

func ensureError(tb testing.TB, err error, contains ...string) {
	tb.Helper()
	if len(contains) == 0 || (len(contains) == 1 && contains[0] == "") {
		if err != nil {
			tb.Fatalf("GOT: %v; WANT: %v", err, contains)
		}
	} else if err == nil {
		tb.Errorf("GOT: %v; WANT: %v", err, contains)
	} else {
		for _, stub := range contains {
			if stub != "" && !strings.Contains(err.Error(), stub) {
				tb.Errorf("GOT: %v; WANT: %q", err, stub)
			}
		}
	}
}

func ensureStringSlicesMatch(tb testing.TB, actual, expected []string) {
	tb.Helper()

	results := make(map[string]int)

	for _, s := range actual {
		results[s] = -1
	}
	for _, s := range expected {
		results[s]++
	}

	keys := make([]string, 0, len(results))
	for k := range results {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, s := range keys {
		v, ok := results[s]
		if !ok {
			panic(fmt.Errorf("cannot find key: %s", s)) // panic because this function is broken
		}
		switch v {
		case -1:
			tb.Errorf("GOT: %q (extra)", s)
		case 0:
			// both slices have this key
		case 1:
			tb.Errorf("WANT: %q (missing)", s)
		default:
			panic(fmt.Errorf("key has invalid value: %s: %d", s, v)) // panic because this function is broken
		}
	}
}

func ensureDirentsMatch(tb testing.TB, actual, expected Dirents) {
	tb.Helper()

	sort.Sort(actual)
	sort.Sort(expected)

	al := len(actual)
	el := len(expected)
	var ai, ei int

	for ai < al || ei < el {
		if ai == al {
			tb.Errorf("GOT: %s %s (extra)", expected[ei].Name(), expected[ei].ModeType())
			ei++
		} else if ei == el {
			tb.Errorf("WANT: %s %s (missing)", actual[ai].Name(), actual[ai].ModeType())
			ai++
		} else if actual[ai].Name() < expected[ei].Name() {
			tb.Errorf("GOT: %s %s (extra)", actual[ai].Name(), actual[ai].ModeType())
			ai++
		} else if expected[ei].Name() < actual[ai].Name() {
			tb.Errorf("WANT: %s %s (missing)", expected[ei].Name(), expected[ei].ModeType())
			ei++
		} else {
			// names match; check mode types
			if got, want := actual[ai].ModeType(), expected[ei].ModeType(); got != want {
				tb.Errorf("GOT: %v; WANT: %v", actual[ai].ModeType(), expected[ei].ModeType())
			}
			ai++
			ei++
		}
	}
}
