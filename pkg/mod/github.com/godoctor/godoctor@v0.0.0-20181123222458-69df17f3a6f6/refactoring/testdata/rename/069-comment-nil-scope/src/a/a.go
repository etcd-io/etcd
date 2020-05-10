package a

// m
type I interface {
	// m
	m()
}

type T int

// m
func (T) m() { //<<<<<rename,12,10,12,10,xyz,pass
}
