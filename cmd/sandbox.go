package main

import (
	"fmt"
	"time"
)

func main() {
	var i int

	for {
		fmt.Println("The current value of i is ", i)
		time.Sleep(1 * time.Minute)
		i++
	}
}
