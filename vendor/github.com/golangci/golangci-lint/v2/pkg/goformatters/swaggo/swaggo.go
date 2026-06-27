package swaggo

import "github.com/golangci/swaggoswag"

const Name = "swaggo"

type Formatter struct {
	formatter *swaggoswag.Formatter
}

func New() *Formatter {
	return &Formatter{
		formatter: swaggoswag.NewFormatter(),
	}
}

func (*Formatter) Name() string {
	return Name
}

func (f *Formatter) Format(path string, src []byte) ([]byte, error) {
	return f.formatter.Format(path, src)
}
