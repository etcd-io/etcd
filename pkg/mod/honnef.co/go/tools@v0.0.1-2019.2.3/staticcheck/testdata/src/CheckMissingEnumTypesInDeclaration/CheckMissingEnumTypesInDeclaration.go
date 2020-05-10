package pkg

const c1 int = 0
const c2 = 0

const (
	c3 int = iota
	c4
	c5
)

const (
	c6 int = 1 // want `only the first constant in this group has an explicit type`
	c7     = 2
	c8     = 3
)

const (
	c9  int = 1
	c10     = 2
	c11     = 3
	c12 int = 4
)

const (
	c13     = 1
	c14 int = 2
	c15 int = 3
	c16 int = 4
)

const (
	c17     = 1
	c18 int = 2
	c19     = 3
	c20 int = 4
)

const (
	c21 int = 1

	c22 = 2
)

const (
	c23 int = 1
	c24 int = 2

	c25 string = "" // want `only the first constant in this group has an explicit type`
	c26        = ""

	c27     = 1
	c28 int = 2

	c29 int = 1
	c30     = 2
	c31 int = 2

	c32 string = "" // want `only the first constant in this group has an explicit type`
	c33        = ""
)

const (
	c34 int = 1 // want `only the first constant in this group has an explicit type`
	c35     = 2

	c36 int = 2
)
