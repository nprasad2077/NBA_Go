package main

import (
	"fmt"
	"github.com/nprasad2077/NBA_Go/utils/security"
)

func main() {
	raw, _ := security.GenerateRawKey()
	fmt.Println(raw)
}