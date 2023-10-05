package utils

import (
	"math/rand"
	"strconv"
)

func GenerateNanoid() string {
	id := ""

	for i := 0; i < 21; i++ {
		num := rand.Intn(10)
		id += strconv.Itoa(num)
	}

	return id
}
