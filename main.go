package main

import (
	"os"

	"orus.io/orus-io/beaver/cmd"
)

func main() {
	if code := cmd.Run(); code != 0 {
		os.Exit(code)
	}
}
