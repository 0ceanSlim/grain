package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server"
)

//go:embed docs/examples/*
var embeddedExamples embed.FS

func main() {

	config.SetEmbeddedExamples(embeddedExamples)

	if err := server.Run(); err != nil {
		fmt.Printf("Application failed: %v\n", err)
		os.Exit(1)
	}
}
