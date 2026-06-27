package internal

const (
	ConstructorCheck Feature = 1 << iota
	StructMethodCheck
	AlphabeticalCheck
	FunctionCheck
)

type Feature uint8

func (f *Feature) Enable(other Feature) {
	*f |= other
}

func (f *Feature) IsEnabled(other Feature) bool {
	return *f&other != 0
}
