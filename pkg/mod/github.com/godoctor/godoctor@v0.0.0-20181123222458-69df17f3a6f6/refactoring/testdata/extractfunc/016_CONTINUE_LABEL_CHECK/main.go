//<<<<<extract,29,1,37,2,printMap,pass
package main

import "fmt"

func main() {
	elements := map[string]map[string]string{
		"H": map[string]string{
			"name":  "Hydrogen",
			"state": "gas",
		},
		"He": map[string]string{
			"name":  "Helium",
			"state": "gas",
		},
		"Li": map[string]string{
			"name":  "Lithium",
			"state": "solid",
		},
		"Be": map[string]string{
			"name":  "Beryllium",
			"state": "solid",
		},
		"Ne": map[string]string{
			"name":  "Neon",
			"state": "gas",
		},
	}
R:
	for _, row := range elements {
		for _, data := range row {
			if data == "Lithium" || data == "solid" || data == "Beryllium" {
				continue R
			}
			fmt.Println(data)
		}
	}
}
