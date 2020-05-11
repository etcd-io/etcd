package pb

import "testing"

func Test_RuneCount(t *testing.T) {
	s := string([]byte{
		27, 91, 51, 49, 109, // {Red}
		72, 101, 108, 108, 111, // Hello
		44, 32, // ,
		112, 108, 97, 121, 103, 114, 111, 117, 110, 100, // Playground
		27, 91, 48, 109, // {Reset}
	})
	if e, l := 17, escapeAwareRuneCountInString(s); l != e {
		t.Errorf("Invalid length %d, expected %d", l, e)
	}
}
