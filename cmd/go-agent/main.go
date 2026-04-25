package main

import (
	"fmt"
	"os"

	"github.com/go-agent/go-agent/internal/cli"
)

func main() {
	if err := cli.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
