package testdata

import "fmt"

func (d *Doctor) SayHello(r *Person) string {
	return fmt.Sprintf("Hello, %v, how are you feeling today?", r.FirstName)
}
