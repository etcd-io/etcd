package undefinedtypes

type SomeStruct struct {
	some.Doer
}

func (c *SomeStruct) Do() {
}
