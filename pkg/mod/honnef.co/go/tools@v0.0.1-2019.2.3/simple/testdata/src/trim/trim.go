package pkg

import (
	"bytes"
	"strings"
)

func foo(s string) int { return 0 }
func gen() string {
	return ""
}

func fn() {
	const s1 = "a string value"
	var s2 = "a string value"
	const n = 14

	var id1 = "a string value"
	var id2 string
	if strings.HasPrefix(id1, s1) { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = id1[len(s1):]
	}

	if strings.HasPrefix(id1, s1) { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = strings.TrimPrefix(id1, s1)
	}

	if strings.HasPrefix(id1, s1) {
		id1 = strings.TrimPrefix(id1, s2)
	}

	if strings.Contains(id1, s1) { // want `should replace.*with.*strings\.Replace`
		id1 = strings.Replace(id1, s1, "something", 123)
	}

	if strings.HasSuffix(id1, s2) { // want `should replace.*with.*strings\.TrimSuffix`
		id1 = id1[:len(id1)-len(s2)]
	}

	var x, y []string
	var i int
	if strings.HasPrefix(x[i], s1) { // want `should replace.*with.*strings\.TrimPrefix`
		x[i] = x[i][len(s1):]
	}

	if strings.HasPrefix(x[i], y[i]) { // want `should replace.*with.*strings\.TrimPrefix`
		x[i] = x[i][len(y[i]):]
	}

	var t struct{ x string }
	if strings.HasPrefix(t.x, s1) { // want `should replace.*with.*strings\.TrimPrefix`
		t.x = t.x[len(s1):]
	}

	if strings.HasPrefix(id1, "test") { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = id1[len("test"):]
	}

	if strings.HasPrefix(id1, "test") { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = id1[4:]
	}

	if strings.HasPrefix(id1, s1) { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = id1[14:]
	}

	if strings.HasPrefix(id1, s1) { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = id1[n:]
	}

	var b1, b2 []byte
	if bytes.HasPrefix(b1, b2) { // want `should replace.*with.*bytes\.TrimPrefix`
		b1 = b1[len(b2):]
	}

	id3 := s2
	if strings.HasPrefix(id1, id3) { // want `should replace.*with.*strings\.TrimPrefix`
		id1 = id1[len(id3):]
	}

	if strings.HasSuffix(id1, s2) {
		id1 = id1[:len(id1)+len(s2)] // wrong operator
	}

	if strings.HasSuffix(id1, s2) {
		id1 = id1[:len(s2)-len(id1)] // wrong math
	}

	if strings.HasSuffix(id1, s2) {
		id1 = id1[:len(id1)-len(id1)] // wrong string length
	}

	if strings.HasPrefix(id1, gen()) {
		id1 = id1[len(gen()):] // dunamic id3
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id1[foo(s1):] // wrong function
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id1[len(id1):] // len() on wrong value
	}

	if strings.HasPrefix(id1, "test") {
		id1 = id1[5:] // wrong length
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id1[len(s1)+1:] // wrong length due to math
	}

	if strings.HasPrefix(id1, s1) {
		id2 = id1[len(s1):] // assigning to the wrong variable
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id1[len(s1):15] // has a max
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id2[len(s1):] // assigning the wrong value
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id1[len(s1):]
		id1 += "" // doing more work in the if
	}

	if strings.HasPrefix(id1, s1) {
		id1 = id1[len(s1):]
	} else {
		id1 = "game over" // else branch
	}

	if strings.HasPrefix(id1, s1) {
		// the conditional is guarding additional code
		id1 = id1[len(s1):]
		println(id1)
	}
}
