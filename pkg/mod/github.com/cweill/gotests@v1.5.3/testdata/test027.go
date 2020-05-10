package testdata

type Opener interface {
	Open() error
}

type Book struct{}

func (*Book) Open() error { return nil }

type door struct{}

func (*door) Open() error { return nil }

type xml struct{}

func (*xml) Open() error { return nil }
