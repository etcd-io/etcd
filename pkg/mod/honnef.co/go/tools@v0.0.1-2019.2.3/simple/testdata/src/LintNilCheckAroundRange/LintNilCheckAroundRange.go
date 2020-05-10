package pkg

import "fmt"

func main() {
	str := []string{}

	// range outside nil check should not match
	for _, s := range str {
		s = s + "B"
	}

	// body with multiple statements should not match
	if str != nil {
		str = append(str, "C")
		for _, s := range str {
			s = s + "D"
		}
	}

	if str != nil { // want `unnecessary nil check around range`
		for _, s := range str {
			s = s + "A"
		}
	}

	var nilMap map[string]int
	if nilMap != nil { // want `unnecessary nil check around range`
		for key, value := range nilMap {
			nilMap[key] = value + 1
		}
	}

	// range over channel can have nil check, as it is required to avoid blocking
	var nilChan chan int
	if nilChan != nil {
		for v := range nilChan {
			fmt.Println(v)
		}
	}
}
