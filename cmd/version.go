package cmd

import (
	"fmt"

	"orus.io/cloudcrane/beaver/lib"
)

// VersionCmd is the "version" command
type VersionCmd struct{}

// Execute the 'version' commands
func (cmd *VersionCmd) Execute([]string) error {
	fmt.Printf("Beaver %q\n", beaver.GetVersion())
	return nil
}

func init() {
	if _, err := parser.AddCommand("version", "Print the program version", "", &VersionCmd{}); err != nil {
		Logger.Fatal().Err(err).Msg("error adding command")
	}
}
