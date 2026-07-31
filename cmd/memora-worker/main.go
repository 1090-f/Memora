package main

import (
	"fmt"

	"github.com/1090-f/Memora/internal/buildinfo"
)

func main() {
	fmt.Println(buildinfo.Info())
}
