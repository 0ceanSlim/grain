package main

import (
	"fmt"
	"os"

	"github.com/0ceanslim/grain/server"
)

func main() {
	if err := server.Run(); err != nil {
		fmt.Printf("Application failed: %v\n", err)
		os.Exit(1)
	}
}