package toml

import (
	"strings"
	"testing"
)

var testParseSmall = `
# This is a TOML document. Boom.

wat = "chipper"

[owner.andrew.gallant] 
hmm = "hi"

[owner] # Whoa there.
andreW = "gallant # poopy" # weeeee
predicate = false
num = -5192
f = -0.5192
zulu = 1979-05-27T07:32:00Z
whoop = "poop"
tests = [ [1, 2, 3], ["abc", "xyz"] ]
arrs = [ # hmm
		 # more comments are awesome.
	1987-07-05T05:45:00Z,
	# say wat?
	1987-07-05T05:45:00Z,
	1987-07-05T05:45:00Z,
	# sweetness
] # more comments
# hehe
`

var testParseSmall2 = `
[a]
better = 43

[a.b.c]
answer = 42
`

func TestParse(t *testing.T) {
	m, err := parse(testParseSmall)
	if err != nil {
		t.Fatal(err)
	}
	printMap(m.mapping, 0)
}

func printMap(m map[string]interface{}, depth int) {
	for k, v := range m {
		testf("%s%s\n", strings.Repeat("  ", depth), k)
		switch subm := v.(type) {
		case map[string]interface{}:
			printMap(subm, depth+1)
		default:
			testf("%s%v\n", strings.Repeat("  ", depth+1), v)
		}
	}
}
