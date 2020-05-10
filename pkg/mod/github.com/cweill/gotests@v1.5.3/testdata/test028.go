package testdata

import "time"

func (c Celsius) ToFahrenheit() Fahrenheit {
	return Fahrenheit(c*1.8 + 32)
}

func HourToSecond(h time.Duration) time.Duration {
	return h * (time.Second / time.Hour)
}
