package testdata

func (name name) Name(n string) string {
	return ""
}

func (name *Name) Name1(n string) string {
	return ""
}

func (n *Name) Name2(name string) string {
	return ""
}

func (n *Name) Name3(nn string) (name string) {
	return ""
}
