package main

import (
	"fmt"
	"os"

	"github.com/mjacobs/agy-reader/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "agy-reader: %v\n", err)
		os.Exit(1)
	}
}
