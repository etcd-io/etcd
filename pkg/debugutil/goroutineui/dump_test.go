// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package goroutineui

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDumpHTML(t *testing.T) {
	now := time.Time{}
	dump := NewDumpFromBytes(now, []byte(fixture))
	dump.SortWaitDesc()  // noop
	dump.SortCountDesc() // noop
	act := dump.HTMLString()

	if false {
		_ = ioutil.WriteFile("test.html", []byte(act), 0644)
	}
	assert.Equal(t, exp, act)
}

// This is the output of debug.PrintStack() on the go playground.
const fixture = `goroutine 1 [running]:
runtime/debug.Stack(0x434070, 0xddb11, 0x0, 0x40e0f8)
	/usr/local/go/src/runtime/debug/stack.go:24 +0xc0
runtime/debug.PrintStack()
	/usr/local/go/src/runtime/debug/stack.go:16 +0x20
main.main()
	/tmp/sandbox157492124/main.go:6 +0x20`

const exp = `<!DOCTYPE html><meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>PanicParse</title>
<link rel="shortcut icon" type="image/png" href="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAIgAAACACAMAAADnN9ENAAAA5FBMVEVMaXFvTjQhISEhISEhISEhISEhISEhISEhISEhISEhISEhISHRbBTRbBTRbBQhISHRbBTRbBTRbBTRbBQ6OjrRbBTZcBTsehbzfhf1fxfidRVOTk75oiL7uCr90zL3kB3/6zv2hhlHMR5OTk5KSkpKSkpRUVFKSkpKSkpMTExBQUE2NjYpKSlOTk5EREQ8PDw4ODgtLS1RUVFQUFAmJiYkJCQ0NDRLS0tTU1MoKCgyMjJHR0dTU1NdXV0rKytXV1dVVVUvLy9eXl5aWlpcXFxcXFxeXl5fX186OjphYWE/Pz9KSkrdB5CTAAAATHRSTlMAEUZggJjf/6/vcL86e1nP2//vmSC7//////9E/////////2WPpYHp////////////////////z/////8w6P////&#43;p/8f///////&#43;/QBb3BQAACWJJREFUeAHslgWC6yAURUOk1N3d3d29&#43;9/SzzxCStvQEfp9zgZycu8DnvTNN78YJCuqqsry77WQNRum2BX0u7JQiYWJw/l7NBz4AderQkFut8frdn&#43;kFBu2wodeYeHxBwjBkFd6StiFOYiboFCAxf9MRXHc9KGpqurDBpqghzcYuCOCeMp21oKelTBVCQt5kDiisXhCJ5aMQkFu6&#43;lg4tDYr2rikRCPKFgQYlGeiYpN7OHbpMj8OgQ8PAGdWIIlnnwbFKYdtgDAJj&#43;MDgZSX/Zwsx4mbyYR6RbntRZVegQDX7/tI1YexMTNObQ&#43;y992EUWRQJIJQjqTzWRyRjtRiMTqKtWQJCTCD4TMaS6bBzLGxLKRKNer1MELX0wEmYHk8pSMGUmIlMI&#43;LC7uTeEQmhEvnZBC/kqGTolfkmSn72NPbFhoWOHswmczeYYC7YaV4MfBHl&#43;BEYmCSJYVSUM3ukgRM5C7g4edqAqLMBq0m1sRh8oeFl4zzp8swtFApXKlmkIkECD8&#43;mpYEZ8iWZGq1YFKKazg582IDiu8bk7Ob5YaOnVCs9UWu&#43;C99D7LWR5fVeY/YpVOp9Po9rp1g25/AIGIXmhp0yMLgSTIhcYDVYaj0ag1Hk/a0yZ1qVVK6KsmfnMVSbMepBkv32M2nw87rfZioatMxsveirqUBLoxHr1CJpvNZmBQSSB&#43;icd6M9dFpttt21CZ4EHfKKny9Ug4awA/kNRmt993loPBAFSICcbtFpRUeuViFIPFiENpp9NYHg6HexW81Suaiaysscc8gsg6jdTxpFNf6oALUaEmBz2SsMBKEkjeL88Bt8VFWj1fCKvWdDolKmYoYDL5ejcSApNoMsZqBH&#43;0byfbiStNEICbFZt/uIN3WtpASWIUwgOiZWgMyPL7v8&#43;tCuIkBdlwW4WWHS/AdyKzRCEfXy7Iw&#43;PHntlti&#43;k0TZ1FKCJJgteV0wHGhr/1Lvp4/HGQvGdJVVVTZyHFl6TGDEIM3FiUIvnrv53zkXyH4PNzm5mknrhUNo7CUjAeSIbGmNW3Vih/gHGKY1jE53uc1BJYSOF4KCmM6YcqaPmvy/86l88MKPbzcXIKLKSwFJFUPMDtpvPDKT63cZKMJSCRglJE4k7x0hjTaZHAOpzjYBIKCpsThpQLSc4D3GaOdWQ0&#43;CEFEo7HSkpIxjzAraXz4RxbURhJQQtKgYSdYE2mPMBtZYWxjMggIRYLKSiFks0Gw9nExjy16XBnxYBxNHghRSTcE65JEXM4rTl2qIOKkRdnIcWXcDgzXktam8s76zBQzOcZ4q6IsIBCSVVhYTmckpJ2HWBA8XoMMEri1oQnJ89Lx&#43;&#43;3cF6U46hYI8CQ4ks4HBzhYdHK0&#43;TDc5ABxDsCCygi4ZpIJYvF0EIGqzsdffvtskschA4wLGHrcrRoCYfDSnBVe&#43;nc5Yis4zAWRwYHFQwpFxKBuEq6Uyt5untBCs8hjB1DCiVnlfDg5PkCdzUT3TmYDINxezqH46jY7&#43;3FxF0Vd75EVcLZdPPCmEH4cFb202RBfIdT2K4ONo5CyQiSE&#43;T5NJvu8q4z/LE/fBYWwsHY/Xgn45NxGEr8SvyDc4R06zt&#43;W0S7wyGrk1MhdJAhkr2TrEXy09ng/toLL2SfcENkMHRoiYVkrERBnKQKriSynzmqORlAlMObjgyHs&#43;G5kSXp5sGV/NjZQmQyKMQOxnNIpBI1m40sCSoJOjgPux0LKW4XQgkOjpqNBwm9wPYtJFGToeNaJaPjbPS2utRh98bvu/1rLZAMEFWISAjxl0RBNkE//Fb2U4sTBI5bkJ2/JBqCFCHfOH27mBMHKXzI4Rpk/1PI8zmkCpnNx3aXaYh1NICkF5BZwKOkYz&#43;1aBUS&#43;OYmsp9aa8i&#43;wWjUjuDc9BqvyPa9AkSW9b5Tg0yNeWm8ItusmkwuniP6wUrHBSTREFmSpk&#43;R7dZMq4l6sh6uP1kdBLc09WQVyLD5Rc1eCGtAkiPk5pLIZDKuiH7EM40hD4R426oqufmlNwHEOs4hSdN7WmQhqYOoLxslEYd81ejTK5C6MWTtIN6SuHPD4fgSOvxCrnznBZxfQtbPrOTi7kyJdkghNyCpMV&#43;NIP31&#43;4gQVsItubg9yzVerqxyePWKhEHWDiLnhpVQAgoCBh08u7qQuyCv69FSKrmQgLLDm3jHEIdXiJ5MIOTxdZ0B4h0cSvzfnFswzh1SiJ5MACSyn7h89iuhJIMEFCoc49KhJxN2aghxlSiJvJdg1nDMnUMV4k0m9Dmysp/3zErOJKDAwrxahnKcCuFkAp6sjP20qauEwzmTgIJAAYbvuCjEhzT/0htkr/VmyX0VCSnOwoABR0EHBnOlkLw55Ct7HTvImUQoFsN4L1rVS0VVSMh9pD/P4pmTYDgiUW98Y4/BPvRgJGnzG1pk698oiX4HblwK5ZC3rHSE31kf7FJOZxtfQgotEmGIQw1GEvLrdzCaJ6WthJKKElAQGs4Y1x0Bv2uYp9E8Ls8kpIhFEI5Bx5SO44nxIcG/9CL7GF2WM5FgPKDAwlBBBs4LHbqQgN&#43;&#43;yCAeJcMzCSiwiCahAgyM5YZjFvZn4Kd4FJdOgo0VCizEwAAG69AOP5Ow9yOrOB6lw2FZniSWIhbJBAphYE&#43;VI&#43;yNEfMVxyZ/o8SjwMKIAgzUoR1MFfpH4EcTx89HiVBgAeaUqTDoGMKhCgl/0TowcZFbCRcFFFokQJwx4FCM0PesrMTE6QISjwKLRBSWIWOho4VCmBdjTL7IheIsxEioEMYVB9/FByYyxtQLSkiBBaFAFML4mWN235&#43;OesaYzcKnwEIMEVCAcd2x4N9rQtMZGFPkXVJoIQYBAgrFUIOJghkcTtJ1ElBgAcZLSYVmSFK1qUHDqbqk0AIMAwQUYNChF4SDCU/HnZxNlxRY3qBhiPAUOsOC33Z3ZTWgxLOAg8AAhWJI2vhLONeEElqoEYJSKAdP7r15FIlgJBqhHVzUtiTLblBKOlqUVIsAx9LQ0aYkGTZlLCYtO3h2iobjKceG56XFPLwYm7pBKYupsRlE39rOk3GZLn51Owpj89VpmyH/lVCkv0LZYCoDjqXtdPoGqf5lQHlaGNTx0L6BeegZJFnmV1djg6OitqPtRKSYZDrTMyrTOuC/BIJbGRhmXKfLktmkk8QwphfQRkA6jz1zIy&#43;PAbsRbInQi8qgF6C4H9PvfQ2E8NXrR7z9tJvf&#43;Z1/ANt&#43;S&#43;GBXoDpAAAAAElFTkSuQmCC"/>
<style>
	body {
		background: black;
		color: lightgray;
	}
	body, pre {
		font-family: Menlo, monospace;
		font-weight: bold;
	}
	.FuncStdLibExported {
		color: #7CFC00;
	}
	.FuncStdLib {
		color: #008000;
	}
	.FuncMain {
		color: #C0C000;
	}
	.FuncOtherExported {
		color: #FF0000;
	}
	.FuncOther {
		color: #A00000;
	}
	.RoutineFirst {
	}
	.Routine {
	}
</style>
<div id="legend">Generated on 0001-01-01 00:00:00 &#43;0000 UTC.
</div>
<div id="content">

	<h1>Running Routine</h1>
	<span class="Routine">1: <span class="state">running</span>
	
	<h2>Stack</h2>
	
	- stack.go:24 <span class="FuncStdLibExported">Stack</span>({[{4407408 } {908049 } {0 } {4251896 }] [] false})<br>
	- stack.go:16 <span class="FuncStdLibExported">PrintStack</span>({[] [] false})<br>
	- .:0 <span class="FuncMain">main</span>({[] [] false})<br>
	

</div>
`
