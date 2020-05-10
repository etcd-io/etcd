package pkg

import (
	"log"
	"regexp"
)

const c1 = `[`
const c2 = `(abc)`

var re1 = regexp.MustCompile(`ab\yef`) // want `error parsing regexp`
var re2 = regexp.MustCompile(c1)       // want `error parsing regexp`
var re3 = regexp.MustCompile(c2)

func fn() {
	_, err := regexp.Compile(`foo(`) // want `error parsing regexp`
	if err != nil {
		panic(err)
	}
	if re2.MatchString("foo(") {
		log.Println("of course 'foo(' matches 'foo('")
	}

	regexp.Match("foo(", nil)       // want `error parsing regexp`
	regexp.MatchReader("foo(", nil) // want `error parsing regexp`
	regexp.MatchString("foo(", "")  // want `error parsing regexp`
}

// must be a basic type to trigger SA4017 (in case of a test failure)
type T string

func (T) Fn() {}

// Don't get confused by methods named init
func (T) init() {}

// this will become a synthetic init function, that we don't want to
// ignore
var _ = regexp.MustCompile("(") // want `error parsing regexp`
