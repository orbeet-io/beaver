package cmd

import (
	"fmt"
)

// VersionCmd is the "version" command
type VersionCmd struct{}

// Execute the 'version' commands
func (cmd *VersionCmd) Execute([]string) error {
	fmt.Printf("Beaver %q\n", Version)
	return nil
}

func init() {
	if _, err := parser.AddCommand("version", "Print the program version", "", &VersionCmd{}); err != nil {
		Logger.Fatal().Err(err).Msg("error adding command")
	}
}
