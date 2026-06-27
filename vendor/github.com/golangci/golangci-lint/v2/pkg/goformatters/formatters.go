package goformatters

type Formatter interface {
	Name() string
	Format(filename string, src []byte) ([]byte, error)
}
