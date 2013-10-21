package toml

import (
	"log"
	"testing"
)

func init() {
	log.SetFlags(0)
}

var testSmall = `
# This is a TOML document. Boom.

[owner] 
[owner] # Whoa there.
andrew = "gallant # poopy" # weeeee
predicate = false
num = -5192
f = -0.5192
zulu = 1979-05-27T07:32:00Z
whoop = "poop"
arrs = [
	1987-07-05T05:45:00Z,
	5,
	"wat?",
	"hehe \n\r kewl",
	[6], [],
	5.0,
	# sweetness
] # more comments
# hehe
`

var testSmaller = `
[a.b] # Do you ignore me?
andrew = "ga# ll\"ant" # what about me?
kait = "brady"
awesomeness = true
pi = 3.14
dob = 1987-07-05T17:45:00Z
perfection = [
	[6, 28],
	[496, 8128]
]
`

func TestLexer(t *testing.T) {
	lx := lex(testSmaller)
	for {
		item := lx.nextItem()
		if item.typ == itemEOF {
			break
		} else if item.typ == itemError {
			t.Fatal(item.val)
		}
		testf("%s\n", item)
	}
}
