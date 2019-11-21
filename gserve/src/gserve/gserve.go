package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Hello from", os.Getenv("ID"))
}
