package main

import (
	"os"
	"orus.io/cloudcrane/beaver/cmd"

)

func main () {
	if code := cmd.Run(); code != 0 {
		os.Exit(code)
	}
}
