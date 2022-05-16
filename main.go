package main

import (
	"orus.io/cloudcrane/beaver/cmd"
	"os"
)

func main() {
	if code := cmd.Run(); code != 0 {
		os.Exit(code)
	}
}
