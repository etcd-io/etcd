package bar

type myNoCopy1 struct{}
type myNoCopy2 struct{}
type locker struct{}            // want `locker`
type someStruct struct{ x int } // want `someStruct`

func (myNoCopy1) Lock()      {}
func (recv myNoCopy2) Lock() {}
func (locker) Lock()         {}
func (locker) Unlock()       {}
func (someStruct) Lock()     {}

type T struct {
	noCopy1 myNoCopy1
	noCopy2 myNoCopy2
	field1  someStruct // want `field1`
	field2  locker     // want `field2`
	field3  int        // want `field3`
}
