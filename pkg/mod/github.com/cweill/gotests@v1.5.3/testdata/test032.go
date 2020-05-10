package testdata

import "strconv"

func (c Celsius) String() string {
	return strconv.Itoa(int(c))
}

func (f Fahrenheit) String() string {
	return strconv.Itoa(int(f))
}
