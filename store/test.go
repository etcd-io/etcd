package store

import (
	"math/rand"
	"strconv"
)

// GenKeys randomly generate num of keys with max depth
func GenKeys(num int, depth int) []string {
	keys := make([]string, num)
	for i := 0; i < num; i++ {

		keys[i] = "/foo/"
		depth := rand.Intn(depth) + 1

		for j := 0; j < depth; j++ {
			keys[i] += "/" + strconv.Itoa(rand.Int())
		}
	}
	return keys
}
