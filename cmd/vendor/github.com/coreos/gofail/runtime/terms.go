// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runtime

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrExhausted = fmt.Errorf("failpoint: terms exhausted")
	ErrBadParse  = fmt.Errorf("failpoint: could not parse terms")
)

// terms encodes the state for a failpoint term string (see fail(9) for examples)
// <fp> :: <term> ( "->" <term> )*
type terms struct {
	// chain is a slice of all the terms from desc
	chain []*term
	// desc is the full term given for the failpoint
	desc string
	// fpath is the failpoint path for these terms
	fpath string

	// mu protects the state of the terms chain
	mu sync.Mutex
}

// term is an executable unit of the failpoint terms chain
type term struct {
	desc string

	mods mod
	act  actFunc
	val  interface{}

	parent *terms
}

type mod interface {
	allow() bool
}

type modCount struct{ c int }

func (mc *modCount) allow() bool {
	if mc.c > 0 {
		mc.c--
		return true
	}
	return false
}

type modProb struct{ p float64 }

func (mp *modProb) allow() bool { return rand.Float64() <= mp.p }

type modList struct{ l []mod }

func (ml *modList) allow() bool {
	for _, m := range ml.l {
		if !m.allow() {
			return false
		}
	}
	return true
}

func newTerms(fpath, desc string) (*terms, error) {
	chain := parse(desc)
	if len(chain) == 0 {
		return nil, ErrBadParse
	}
	t := &terms{chain: chain, desc: desc, fpath: fpath}
	for _, c := range chain {
		c.parent = t
	}
	return t, nil
}

func (t *terms) String() string { return t.desc }

func (t *terms) eval() (interface{}, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, term := range t.chain {
		if term.mods.allow() {
			return term.do(), nil
		}
	}
	return nil, nil
}

// split terms from a -> b -> ... into [a, b, ...]
func parse(desc string) (chain []*term) {
	origDesc := desc
	for len(desc) != 0 {
		t := parseTerm(desc)
		if t == nil {
			fmt.Printf("failed to parse %q past %q\n", origDesc, desc)
			return nil
		}
		desc = desc[len(t.desc):]
		chain = append(chain, t)
		if len(desc) >= 2 {
			if !strings.HasPrefix(desc, "->") {
				fmt.Printf("failed to parse %q past %q, expected \"->\"\n", origDesc, desc)
				return nil
			}
			desc = desc[2:]
		}
	}
	return chain
}

// <term> :: <mod> <act> [ "(" <val> ")" ]
func parseTerm(desc string) *term {
	t := &term{}
	modStr, mods := parseMod(desc)
	t.mods = &modList{mods}
	actStr, act := parseAct(desc[len(modStr):])
	t.act = act
	valStr, val := parseVal(desc[len(modStr)+len(actStr):])
	t.val = val
	t.desc = desc[:len(modStr)+len(actStr)+len(valStr)]
	if len(t.desc) == 0 {
		return nil
	}
	return t
}

// <mod> :: ((<float> "%")|(<int> "*" ))*
func parseMod(desc string) (ret string, mods []mod) {
	for {
		s, v := parseIntFloat(desc)
		if len(s) == 0 {
			break
		}
		if len(s) == len(desc) {
			return "", nil
		}
		switch v := v.(type) {
		case float64:
			if desc[len(s)] != '%' {
				return "", nil
			}
			ret = ret + desc[:len(s)+1]
			mods = append(mods, &modProb{v / 100.0})
			desc = desc[len(s)+1:]
		case int:
			if desc[len(s)] != '*' {
				return "", nil
			}
			ret = ret + desc[:len(s)+1]
			mods = append(mods, &modCount{v})
			desc = desc[len(s)+1:]
		default:
			panic("???")
		}
	}
	return ret, mods
}

// parseIntFloat parses an int or float from a string and returns the string
// it parsed it from (unlike scanf).
func parseIntFloat(desc string) (string, interface{}) {
	// parse for ints
	i := 0
	for i < len(desc) {
		if desc[i] < '0' || desc[i] > '9' {
			break
		}
		i++
	}
	if i == 0 {
		return "", nil
	}

	intVal := int(0)
	_, err := fmt.Sscanf(desc[:i], "%d", &intVal)
	if err != nil {
		return "", nil
	}
	if len(desc) == i {
		return desc[:i], intVal
	}
	if desc[i] != '.' {
		return desc[:i], intVal
	}

	// parse for floats
	i++
	if i == len(desc) {
		return desc[:i], float64(intVal)
	}

	j := i
	for i < len(desc) {
		if desc[i] < '0' || desc[i] > '9' {
			break
		}
		i++
	}
	if j == i {
		return desc[:i], float64(intVal)
	}

	f := float64(0)
	if _, err = fmt.Sscanf(desc[:i], "%f", &f); err != nil {
		return "", nil
	}
	return desc[:i], f
}

// parseAct parses an action
// <act> :: "off" | "return" | "sleep" | "panic" | "break" | "print"
func parseAct(desc string) (string, actFunc) {
	for k, v := range actMap {
		if strings.HasPrefix(desc, k) {
			return k, v
		}
	}
	return "", nil
}

// <val> :: <int> | <string> | <bool> | <nothing>
func parseVal(desc string) (string, interface{}) {
	// return => struct{}
	if len(desc) == 0 {
		return "", struct{}{}
	}
	// malformed
	if len(desc) == 1 || desc[0] != '(' {
		return "", nil
	}
	// return() => struct{}
	if desc[1] == ')' {
		return "()", struct{}{}
	}
	// return("s") => string
	s := ""
	n, err := fmt.Sscanf(desc[1:], "%q", &s)
	if n == 1 && err == nil {
		return desc[:len(s)+4], s
	}
	// return(1) => int
	v := 0
	n, err = fmt.Sscanf(desc[1:], "%d", &v)
	if n == 1 && err == nil {
		return desc[:len(fmt.Sprintf("%d", v))+2], v
	}
	// return(true) => bool
	b := false
	n, err = fmt.Sscanf(desc[1:], "%t", &b)
	if n == 1 && err == nil {
		return desc[:len(fmt.Sprintf("%t", b))+2], b
	}
	// unknown type; malformed input?
	return "", nil
}

type actFunc func(*term) interface{}

var actMap = map[string]actFunc{
	"off":    actOff,
	"return": actReturn,
	"sleep":  actSleep,
	"panic":  actPanic,
	"break":  actBreak,
	"print":  actPrint,
}

func (t *term) do() interface{} { return t.act(t) }

func actOff(t *term) interface{} { return nil }

func actReturn(t *term) interface{} { return t.val }

func actSleep(t *term) interface{} {
	var dur time.Duration
	switch v := t.val.(type) {
	case int:
		dur = time.Duration(v) * time.Millisecond
	case string:
		vDur, err := time.ParseDuration(v)
		if err != nil {
			fmt.Printf("failpoint: could not parse sleep(%v) on %s\n", v, t.parent.fpath)
			return nil
		}
		dur = vDur
	default:
		fmt.Printf("failpoint: ignoring sleep(%v) on %s\n", v, t.parent.fpath)
		return nil
	}
	time.Sleep(dur)
	return nil
}

func actPanic(t *term) interface{} {
	if t.val != nil {
		panic(fmt.Sprintf("failpoint panic: %v", t.val))
	}
	panic("failpoint panic: " + t.parent.fpath)
}

func actBreak(t *term) interface{} {
	p, perr := exec.LookPath(os.Args[0])
	if perr != nil {
		panic(perr)
	}
	cmd := exec.Command("gdb", p, fmt.Sprintf("%d", os.Getpid()))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}

	// wait for gdb prompt
	// XXX: tried doing this by piping stdout here and waiting on "(gdb) "
	// but the the output won't appear since the process is STOPed and
	// can't copy it back to the actual stdout
	time.Sleep(3 * time.Second)

	// don't zombie gdb
	go cmd.Wait()
	return nil
}

func actPrint(t *term) interface{} {
	fmt.Println("failpoint print:", t.parent.fpath)
	return nil
}
