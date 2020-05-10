package pkg

import "fmt"

func gen() interface{} { return nil }

func fn(x, y interface{}) {
	switch z := x.(type) {
	case int:
		_ = z
		fmt.Println(x.(int))
	}
	switch x.(type) {
	case int:
		fmt.Println(x.(int), y.(int))
	}
	switch x.(type) { // want `assigning the result of this type assertion`
	case int:
		fmt.Println(x.(int))
	}
	switch x.(type) {
	case int:
		fmt.Println(x.(string))
	}
	switch x.(type) {
	case int:
		fmt.Println(y.(int))
	}
	switch (gen()).(type) {
	case int:
		fmt.Println((gen()).(int))
	}
}
