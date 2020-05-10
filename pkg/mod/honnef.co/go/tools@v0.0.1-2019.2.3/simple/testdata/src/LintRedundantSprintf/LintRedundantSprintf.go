package pkg

import "fmt"

type T1 string
type T2 T1
type T3 int
type T4 int
type T5 int
type T6 string

func (T3) String() string        { return "" }
func (T6) String() string        { return "" }
func (T4) String(arg int) string { return "" }
func (T5) String()               {}

func fn() {
	var t1 T1
	var t2 T2
	var t3 T3
	var t4 T4
	var t5 T5
	var t6 T6
	_ = fmt.Sprintf("%s", "test")      // want `is already a string`
	_ = fmt.Sprintf("%s", t1)          // want `is a string`
	_ = fmt.Sprintf("%s", t2)          // want `is a string`
	_ = fmt.Sprintf("%s", t3)          // want `should use String\(\) instead of fmt\.Sprintf`
	_ = fmt.Sprintf("%s", t3.String()) // want `is already a string`
	_ = fmt.Sprintf("%s", t4)
	_ = fmt.Sprintf("%s", t5)
	_ = fmt.Sprintf("%s %s", t1, t2)
	_ = fmt.Sprintf("%v", t1)
	_ = fmt.Sprintf("%s", t6) // want `should use String\(\) instead of fmt\.Sprintf`
}
