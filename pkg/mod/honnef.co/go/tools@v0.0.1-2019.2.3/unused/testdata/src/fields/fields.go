// Test of field usage detection

package pkg

type t1 struct{ f11, f12 int }
type t2 struct{ f21, f22 int }
type t3 struct{ f31 t4 }
type t4 struct{ f41 int }
type t5 struct{ f51 int }
type t6 struct{ f61 int }
type t7 struct{ f71 int }
type m1 map[string]t7
type t8 struct{ f81 int }
type t9 struct{ f91 int }
type t10 struct{ f101 int }
type t11 struct{ f111 int }
type s1 []t11
type t12 struct{ f121 int }
type s2 []t12
type t13 struct{ f131 int }
type t14 struct{ f141 int }
type a1 [1]t14
type t15 struct{ f151 int }
type a2 [1]t15
type t16 struct{ f161 int }
type t17 struct{ f171, f172 int }       // want `t17`
type t18 struct{ f181, f182, f183 int } // want `f182` `f183`

type t19 struct{ f191 int }
type m2 map[string]t19

type t20 struct{ f201 int }
type m3 map[string]t20

type t21 struct{ f211, f212 int } // want `f211`

func foo() {
	_ = t10{1}
	_ = t21{f212: 1}
	_ = []t1{{1, 2}}
	_ = t2{1, 2}
	_ = []struct{ a int }{{1}}

	// XXX
	// _ = []struct{ foo struct{ bar int } }{{struct{ bar int }{1}}}

	_ = []t1{t1{1, 2}}
	_ = []t3{{t4{1}}}
	_ = map[string]t5{"a": {1}}
	_ = map[t6]string{{1}: "a"}
	_ = m1{"a": {1}}
	_ = map[t8]t8{{}: {1}}
	_ = map[t9]t9{{1}: {}}
	_ = s1{{1}}
	_ = s2{2: {1}}
	_ = [...]t13{{1}}
	_ = a1{{1}}
	_ = a2{0: {1}}
	_ = map[[1]t16]int{{{1}}: 1}
	y := struct{ x int }{}
	_ = y
	_ = t18{f181: 1}
	_ = []m2{{"a": {1}}}
	_ = [][]m3{{{"a": {1}}}}
}

func init() { foo() }
