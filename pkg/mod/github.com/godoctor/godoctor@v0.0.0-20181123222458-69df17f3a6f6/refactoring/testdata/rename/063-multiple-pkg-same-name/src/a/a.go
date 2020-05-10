package a

type I interface {
	m()
}

type T int

func (T) m() { //<<<<<rename,9,10,9,10,xyz,pass
}
