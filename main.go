package main

import (
	"os"

	"bundlecheck/cmd"
)

func main() { os.Exit(cmd.Execute(os.Args[1:], os.Stdout, os.Stderr)) }
