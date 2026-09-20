package main

import (
	"os"

	"github.com/sonuKumar03/bundlecheck/cmd"
)

func main() { os.Exit(cmd.Execute(os.Args[1:], os.Stdout, os.Stderr)) }
